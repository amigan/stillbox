package incstore

import (
	"context"
	"time"

	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/internal/jsontypes"
	"dynatron.me/x/stillbox/pkg/auth"
	"dynatron.me/x/stillbox/pkg/calls"
	"dynatron.me/x/stillbox/pkg/database"
	"dynatron.me/x/stillbox/pkg/incidents"
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
	Incidents(ctx context.Context, p IncidentsParams) (incs []incidents.Incident, totalCount int, err error)

	// Incident gets a single incident.
	Incident(ctx context.Context, id uuid.UUID) (*incidents.Incident, error)

	// UpdateIncident updates an incident.
	UpdateIncident(ctx context.Context, id uuid.UUID, p UpdateIncidentParams) (*incidents.Incident, error)

	// DeleteIncident deletes an incident.
	DeleteIncident(ctx context.Context, id uuid.UUID) error
}

type store struct {
}

type storeCtxKey string

const StoreCtxKey storeCtxKey = "store"

func CtxWithStore(ctx context.Context, s Store) context.Context {
	return context.WithValue(ctx, StoreCtxKey, s)
}

func FromCtx(ctx context.Context) Store {
	s, ok := ctx.Value(StoreCtxKey).(Store)
	if !ok {
		return NewStore()
	}

	return s
}

func NewStore() Store {
	return &store{}
}

func (s *store) CreateIncident(ctx context.Context, inc incidents.Incident) (*incidents.Incident, error) {
	db := database.FromCtx(ctx)
	var dbInc database.Incident

	id := uuid.New()

	txErr := db.InTx(ctx, func(db database.Store) error {
		var err error
		dbInc, err = db.CreateIncident(ctx, database.CreateIncidentParams{
			ID:          id,
			Name:        inc.Name,
			Description: inc.Description,
			StartTime:   inc.StartTime.PGTypeTSTZ(),
			EndTime:     inc.EndTime.PGTypeTSTZ(),
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

func (s *store) AddRemoveIncidentCalls(ctx context.Context, incidentID uuid.UUID, addCallIDs []uuid.UUID, notes []byte, removeCallIDs []uuid.UUID) error {
	return database.FromCtx(ctx).InTx(ctx, func(db database.Store) error {
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

func (s *store) Incidents(ctx context.Context, p IncidentsParams) (incs []incidents.Incident, totalCount int, err error) {
	db := database.FromCtx(ctx)

	offset, perPage := p.Pagination.OffsetPerPage(100)
	dbParam := database.ListIncidentsPParams{
		Start:     p.Start.PGTypeTSTZ(),
		End:       p.End.PGTypeTSTZ(),
		Filter:    p.Filter,
		Direction: p.Direction.DirString(common.DirAsc),
		Offset:    offset,
		PerPage:   perPage,
	}

	var count int64
	var rows []database.Incident
	txErr := db.InTx(ctx, func(db database.Store) error {
		var err error
		count, err = db.ListIncidentsCount(ctx, dbParam.Start, dbParam.End)
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

	incs = make([]incidents.Incident, 0, len(rows))
	for _, v := range rows {
		incs = append(incs, fromDBIncident(v.ID, v))
	}

	return incs, int(count), err
}

func fromDBIncident(id uuid.UUID, d database.Incident) incidents.Incident {
	return incidents.Incident{
		ID:          id,
		Name:        d.Name,
		Description: d.Description,
		StartTime:   jsontypes.TimePtrFromTSTZ(d.StartTime),
		EndTime:     jsontypes.TimePtrFromTSTZ(d.EndTime),
		Metadata:    d.Metadata,
	}
}

func fromDBCalls(d []database.GetIncidentCallsRow) []incidents.IncidentCall {
	r := make([]incidents.IncidentCall, 0, len(d))
	for _, v := range d {
		dur := calls.CallDuration(time.Duration(common.ZeroIfNil(v.Duration)) * time.Millisecond)
		sub := common.PtrTo(auth.UserID(common.ZeroIfNil(v.Submitter)))
		r = append(r, incidents.IncidentCall{
			Call: calls.Call{
				ID:          v.CallID,
				AudioName:   common.ZeroIfNil(v.AudioName),
				AudioType:   common.ZeroIfNil(v.AudioType),
				Duration:    dur,
				DateTime:    v.CallDate.Time,
				Frequencies: v.Frequencies,
				Frequency:   v.Frequency,
				Patches:     v.Patches,
				Source:      v.Source,
				System:      v.System,
				Submitter:   sub,
				Talkgroup:   v.Talkgroup,
			},
			Notes: v.Notes,
		})
	}

	return r
}

func (s *store) Incident(ctx context.Context, id uuid.UUID) (*incidents.Incident, error) {
	var r incidents.Incident
	txErr := database.FromCtx(ctx).InTx(ctx, func(db database.Store) error {
		inc, err := db.GetIncident(ctx, id)
		if err != nil {
			return err
		}

		calls, err := db.GetIncidentCalls(ctx, id)
		if err != nil {
			return err
		}

		r = fromDBIncident(id, inc)
		r.Calls = fromDBCalls(calls)

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
		StartTime:   uip.StartTime.PGTypeTSTZ(),
		EndTime:     uip.EndTime.PGTypeTSTZ(),
		Location:    uip.Location,
		Metadata:    uip.Metadata,
	}
}

func (s *store) UpdateIncident(ctx context.Context, id uuid.UUID, p UpdateIncidentParams) (*incidents.Incident, error) {
	db := database.FromCtx(ctx)

	dbInc, err := db.UpdateIncident(ctx, p.toDBUIP(id))
	if err != nil {
		return nil, err
	}

	inc := fromDBIncident(id, dbInc)

	return &inc, nil
}

func (s *store) DeleteIncident(ctx context.Context, id uuid.UUID) error {
	return database.FromCtx(ctx).DeleteIncident(ctx, id)
}

func (s *store) UpdateNotes(ctx context.Context, incidentID uuid.UUID, callID uuid.UUID, notes []byte) error {
	return database.FromCtx(ctx).UpdateCallIncidentNotes(ctx, notes, incidentID, callID)
}
