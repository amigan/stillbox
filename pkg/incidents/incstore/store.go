package incstore

import (
	"context"
	"time"

	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/internal/jsontypes"
	"dynatron.me/x/stillbox/pkg/authz"
	"dynatron.me/x/stillbox/pkg/authz/entities"
	"dynatron.me/x/stillbox/pkg/calls"
	"dynatron.me/x/stillbox/pkg/database"
	"dynatron.me/x/stillbox/pkg/incidents"
	"dynatron.me/x/stillbox/pkg/services"
	"dynatron.me/x/stillbox/pkg/talkgroups"
	"dynatron.me/x/stillbox/pkg/users"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type IncidentsParams struct {
	common.Pagination
	Direction *common.SortDirection `json:"dir"`
	Filter    *string               `json:"filter"`

	Start *jsontypes.Time `json:"start"`
	End   *jsontypes.Time `json:"end"`
}

type Store interface {
	// CreateIncident creates an incident.
	CreateIncident(ctx context.Context, inc incidents.Incident) (*incidents.Incident, error)

	// AddToIncident adds the specified call IDs to an incident.
	// If not nil, notes must be valid json.
	AddRemoveIncidentCalls(ctx context.Context, incidentID uuid.UUID, addCallIDs []uuid.UUID, notes []byte, removeCallIDs []uuid.UUID) error

	// UpdateNotes updates the notes for a call-incident mapping.
	UpdateNotes(ctx context.Context, incidentID uuid.UUID, callID uuid.UUID, notes []byte) error

	// Incidents gets incidents matching parameters and pagination.
	Incidents(ctx context.Context, p IncidentsParams) (incs []Incident, totalCount int, err error)

	// Incident gets a single incident.
	Incident(ctx context.Context, id uuid.UUID, opts ...IncidentOption) (*incidents.Incident, error)

	// IncidentCalls gets the calls in an incident.
	IncidentCalls(ctx context.Context, id uuid.UUID, pag *CallsFilter) (*incidents.IncidentCalls, error)

	// UpdateIncident updates an incident.
	UpdateIncident(ctx context.Context, id uuid.UUID, p UpdateIncidentParams) (*incidents.Incident, error)

	// DeleteIncident deletes an incident.
	DeleteIncident(ctx context.Context, id uuid.UUID) error

	// Owner returns an incident with only the owner filled out.
	Owner(ctx context.Context, id uuid.UUID) (incidents.Incident, error)

	// CallIn returns whether an incident is in an call
	CallIn(ctx context.Context, inc uuid.UUID, call uuid.UUID) (bool, error)

	// TGsIn returns the talkgroups referenced by an incident as a map, primarily for rbac use.
	TGsIn(ctx context.Context, inc uuid.UUID) (talkgroups.PresenceMap, error)
}

type postgresStore struct {
	db database.Store
}

type storeCtxKey string

const StoreCtxKey storeCtxKey = "store"

func CtxWithStore(ctx context.Context, s Store) context.Context {
	return services.WithValue(ctx, StoreCtxKey, s)
}

func FromCtx(ctx context.Context) Store {
	s, ok := services.Value(ctx, StoreCtxKey).(Store)
	if !ok {
		panic("no incident store in context")
	}

	return s
}

func NewStore(db database.Store) Store {
	return &postgresStore{db: db}
}

func (s *postgresStore) CreateIncident(ctx context.Context, inc incidents.Incident) (*incidents.Incident, error) {
	user, err := users.UserCheck(ctx, new(incidents.Incident), "create")
	if err != nil {
		return nil, err
	}

	var dbInc database.Incident

	id := uuid.New()

	txErr := s.db.InTx(ctx, func(db database.Store) error {
		var err error
		dbInc, err = db.CreateIncident(ctx, database.CreateIncidentParams{
			ID:          id,
			OwnerID:     user.ID.Int(),
			Name:        inc.Name,
			Description: inc.Description,
			StartTime:   (*time.Time)(inc.StartTime),
			EndTime:     (*time.Time)(inc.EndTime),
			Location:    inc.Location.RawMessage,
			Metadata:    inc.Metadata,
		})
		if err != nil {
			return err
		}

		if len(inc.Calls) > 0 {
			callIDs := make([]uuid.UUID, 0, len(inc.Calls))
			notes := make([][]byte, 0, len(inc.Calls))
			hasNote := false
			for _, c := range inc.Calls {
				callIDs = append(callIDs, c.ID)
				if c.Notes != nil {
					hasNote = true
				}
				notes = append(notes, c.Notes)
			}
			if !hasNote {
				notes = nil
			}

			err = db.AddToIncident(ctx, dbInc.ID, callIDs, notes)
			if err != nil {
				return err
			}
		}

		return nil
	}, pgx.TxOptions{})
	if txErr != nil {
		return nil, txErr
	}

	inc = fromDBIncident(id, dbInc)

	return &inc, nil
}

func (s *postgresStore) AddRemoveIncidentCalls(ctx context.Context, incidentID uuid.UUID, addCallIDs []uuid.UUID, notes []byte, removeCallIDs []uuid.UUID) error {
	inc, err := s.Owner(ctx, incidentID)
	if err != nil {
		return err
	}

	_, err = authz.Check(ctx, &inc, authz.WithActions(entities.ActionUpdate))
	if err != nil {
		return err
	}

	return s.db.InTx(ctx, func(db database.Store) error {
		if len(addCallIDs) > 0 {
			var noteAr [][]byte
			if notes != nil {
				noteAr = make([][]byte, len(addCallIDs))
				for i := range addCallIDs {
					noteAr[i] = notes
				}
			}

			err := db.AddToIncident(ctx, incidentID, addCallIDs, noteAr)
			if err != nil {
				return err
			}
		}

		if len(removeCallIDs) > 0 {
			err := db.RemoveFromIncident(ctx, incidentID, removeCallIDs)
			if err != nil {
				return err
			}
		}

		return nil
	}, pgx.TxOptions{})
}

func (s *postgresStore) Incidents(ctx context.Context, p IncidentsParams) (incs []Incident, totalCount int, err error) {
	_, err = authz.Check(ctx, new(incidents.Incident), authz.WithActions(entities.ActionRead))
	if err != nil {
		return nil, 0, err
	}

	offset, perPage := p.Pagination.OffsetPerPage(100)
	dbParam := database.ListIncidentsPParams{
		Start:     (*time.Time)(p.Start),
		End:       (*time.Time)(p.End),
		Filter:    p.Filter,
		Direction: p.Direction.DirString(common.DirAsc),
		Offset:    offset,
		PerPage:   perPage,
	}

	var count int64
	var rows []database.ListIncidentsPRow
	txErr := s.db.InTx(ctx, func(db database.Store) error {
		var err error
		count, err = db.ListIncidentsCount(ctx, dbParam.Start, dbParam.End, dbParam.Filter)
		if err != nil {
			return err
		}

		if offset > int32(count) {
			return common.ErrPageOutOfRange
		}

		rows, err = db.ListIncidentsP(ctx, dbParam)
		return err
	}, pgx.TxOptions{})
	if txErr != nil {
		return nil, 0, txErr
	}

	incs = make([]Incident, 0, len(rows))
	for _, v := range rows {
		incs = append(incs, fromDBListInPRow(v.ID, v))
	}

	return incs, int(count), err
}

func fromDBIncident(id uuid.UUID, d database.Incident) incidents.Incident {
	return incidents.Incident{
		ID:          id,
		OwnerID:     users.UserID(d.OwnerID),
		Name:        d.Name,
		Description: d.Description,
		CreatedAt:   (*jsontypes.Time)(d.CreatedAt),
		StartTime:   (*jsontypes.Time)(d.StartTime),
		EndTime:     (*jsontypes.Time)(d.EndTime),
		Metadata:    d.Metadata,
	}
}

type Incident struct {
	incidents.Incident

	CallCount int `json:"callCount"`
}

type CallsFilter struct {
	common.Pagination
}

func (s *postgresStore) IncidentCalls(ctx context.Context, id uuid.UUID, pag *CallsFilter) (*incidents.IncidentCalls, error) {
	_, err := authz.Check(ctx, &incidents.Incident{ID: id}, authz.WithActions(entities.ActionRead))
	if err != nil {
		return nil, err
	}

	var ic incidents.IncidentCalls
	txErr := s.db.InTx(ctx, func(db database.Store) error {
		count, err := db.GetIncidentCallCount(ctx, id)
		if err != nil {
			return err
		}

		var offset int32
		var perPage *int32

		if pag != nil {
			os, pp := pag.OffsetPerPage(25)
			offset = os
			perPage = &pp
		}

		calls, err := db.GetIncidentCalls(ctx, id, offset, perPage)
		if err != nil {
			return err
		}

		ic = incidents.IncidentCalls{
			Calls: fromDBCalls(calls),
			Count: int(count),
		}

		return nil
	}, pgx.TxOptions{})
	if txErr != nil {
		return nil, txErr
	}

	return &ic, nil
}

func fromDBListInPRow(id uuid.UUID, d database.ListIncidentsPRow) Incident {
	return Incident{
		Incident: incidents.Incident{
			ID:          id,
			OwnerID:     users.UserID(d.OwnerID),
			Owner:       d.Owner,
			Name:        d.Name,
			Description: d.Description,
			CreatedAt:   (*jsontypes.Time)(d.CreatedAt),
			StartTime:   (*jsontypes.Time)(d.StartTime),
			EndTime:     (*jsontypes.Time)(d.EndTime),
			Metadata:    d.Metadata,
		},
		CallCount: int(d.CallsCount),
	}
}

func fromDBCalls(d []database.GetIncidentCallsRow) []incidents.IncidentCall {
	r := make([]incidents.IncidentCall, 0, len(d))
	for _, v := range d {
		dur := calls.CallDuration(time.Duration(common.ZeroIfNil(v.Duration)) * time.Millisecond)
		sub := common.PtrTo(users.UserID(common.ZeroIfNil(v.Submitter)))
		r = append(r, incidents.IncidentCall{
			Call: calls.Call{
				ID:          v.CallID,
				AudioName:   common.ZeroIfNil(v.AudioName),
				AudioType:   string(v.AudioType.AudioMIME),
				Duration:    dur,
				DateTime:    common.ZeroIfNil(v.CallDate),
				Frequencies: v.Frequencies,
				Frequency:   v.Frequency,
				Patches:     v.Patches,
				TalkerAlias: v.TalkerAlias,
				Source:      v.Source,
				System:      v.SystemID,
				Submitter:   sub,
				Talkgroup:   v.TGID,
				Transcript:  v.Transcript,
			},
			Notes: v.Notes,
		})
	}

	return r
}

type incidentOptions struct {
	withoutCalls bool
}

type IncidentOption func(*incidentOptions)

func WithoutCalls() IncidentOption {
	return func(o *incidentOptions) {
		o.withoutCalls = true
	}
}

func (s *postgresStore) Incident(ctx context.Context, id uuid.UUID, opts ...IncidentOption) (*incidents.Incident, error) {
	_, err := authz.Check(ctx, &incidents.Incident{ID: id}, authz.WithActions(entities.ActionRead))
	if err != nil {
		return nil, err
	}

	var options incidentOptions
	for _, opt := range opts {
		opt(&options)
	}

	var r incidents.Incident
	txErr := s.db.InTx(ctx, func(db database.Store) error {
		inc, err := db.GetIncident(ctx, id)
		if err != nil {
			return err
		}

		r = fromDBIncident(id, inc.Incident)
		r.Owner = inc.Owner

		if !options.withoutCalls {
			calls, err := db.GetIncidentCalls(ctx, id, 0, nil)
			if err != nil {
				return err
			}
			r.Calls = fromDBCalls(calls)
		}

		return nil
	}, pgx.TxOptions{})
	if txErr != nil {
		return nil, txErr
	}

	return &r, nil
}

type UpdateIncidentParams struct {
	Name        *string            `json:"name"`
	Description *string            `json:"description"`
	StartTime   *jsontypes.Time    `json:"startTime"`
	EndTime     *jsontypes.Time    `json:"endTime"`
	Location    []byte             `json:"location"`
	Metadata    jsontypes.Metadata `json:"metadata"`
}

func (uip UpdateIncidentParams) toDBUIP(id uuid.UUID) database.UpdateIncidentParams {
	return database.UpdateIncidentParams{
		ID:          id,
		Name:        uip.Name,
		Description: uip.Description,
		StartTime:   (*time.Time)(uip.StartTime),
		EndTime:     (*time.Time)(uip.EndTime),
		Location:    uip.Location,
		Metadata:    uip.Metadata,
	}
}

func (s *postgresStore) UpdateIncident(ctx context.Context, id uuid.UUID, p UpdateIncidentParams) (*incidents.Incident, error) {
	ckinc, err := s.Owner(ctx, id)
	if err != nil {
		return nil, err
	}

	_, err = authz.Check(ctx, &ckinc, authz.WithActions(entities.ActionUpdate))
	if err != nil {
		return nil, err
	}

	dbInc, err := s.db.UpdateIncident(ctx, p.toDBUIP(id))
	if err != nil {
		return nil, err
	}

	inc := fromDBIncident(id, dbInc)

	return &inc, nil
}

func (s *postgresStore) DeleteIncident(ctx context.Context, id uuid.UUID) error {
	inc, err := s.Owner(ctx, id)
	if err != nil {
		return err
	}

	_, err = authz.Check(ctx, &inc, authz.WithActions(entities.ActionDelete))
	if err != nil {
		return err
	}

	return s.db.DeleteIncident(ctx, id)
}

func (s *postgresStore) UpdateNotes(ctx context.Context, incidentID uuid.UUID, callID uuid.UUID, notes []byte) error {
	return s.db.UpdateCallIncidentNotes(ctx, notes, incidentID, callID)
}

func (s *postgresStore) Owner(ctx context.Context, id uuid.UUID) (incidents.Incident, error) {
	owner, err := s.db.GetIncidentOwner(ctx, id)
	return incidents.Incident{ID: id, OwnerID: users.UserID(owner)}, err
}

func (s *postgresStore) CallIn(ctx context.Context, inc uuid.UUID, call uuid.UUID) (bool, error) {
	return s.db.CallInIncident(ctx, inc, call)
}

func (s *postgresStore) TGsIn(ctx context.Context, id uuid.UUID) (talkgroups.PresenceMap, error) {
	_, err := authz.Check(ctx, &incidents.Incident{ID: id}, authz.WithActions(entities.ActionRead))
	if err != nil {
		return nil, err
	}

	tgs, err := s.db.GetIncidentTalkgroups(ctx, id)
	if err != nil {
		return nil, err
	}

	m := make(talkgroups.PresenceMap, len(tgs))
	for _, t := range tgs {
		m.Put(talkgroups.ID{
			System:    uint32(t.System),
			Talkgroup: uint32(t.Talkgroup),
		})
	}

	return m, nil
}
