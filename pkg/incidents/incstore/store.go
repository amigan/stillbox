package incstore

import (
	"context"

	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/internal/jsontypes"
	"dynatron.me/x/stillbox/pkg/database"
	"dynatron.me/x/stillbox/pkg/incidents"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type IncidentsParams struct {
	common.Pagination
	Direction *common.SortDirection `json:"dir"`

	Start *jsontypes.Time `json:"start"`
	End   *jsontypes.Time `json:"end"`
}

type Store interface {
	// CreateIncident creates an incident.
	CreateIncident(ctx context.Context, inc incidents.Incident) (database.Incident, error)

	// AddToIncident adds the specified call IDs to an incident.
	// If not nil, notes must be valid json.
	AddToIncident(ctx context.Context, incidentID uuid.UUID, callIDs []uuid.UUID, notes []byte) error

	// Incidents gets incidents matching parameters and pagination.
	Incidents(ctx context.Context, p IncidentsParams) (incs []database.Incident, totalCount int, err error)

	// Incident gets a single incident.
	Incident(ctx context.Context, id uuid.UUID) (database.Incident, error)

	// UpdateIncident updates an incident.
	UpdateIncident(ctx context.Context, id uuid.UUID, p UpdateIncidentParams) (database.Incident, error)

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

func (s *store) CreateIncident(ctx context.Context, inc incidents.Incident) (database.Incident, error) {
	db := database.FromCtx(ctx)
	dbInc, err := db.CreateIncident(ctx, database.CreateIncidentParams{
		ID:          uuid.New(),
		Name:        inc.Name,
		Description: inc.Description,
		StartTime:   inc.StartTime.PGTypeTSTZ(),
		EndTime:     inc.EndTime.PGTypeTSTZ(),
		Location:    inc.Location.RawMessage,
		Metadata:    inc.Metadata,
	})

	return dbInc, err
}

func (s *store) AddToIncident(ctx context.Context, incidentID uuid.UUID, callIDs []uuid.UUID, notes []byte) error {
	db := database.FromCtx(ctx)

	var noteAr [][]byte
	if notes != nil {
		noteAr = make([][]byte, len(callIDs))
		for i := range callIDs {
			noteAr[i] = notes
		}
	}

	return db.AddToIncident(ctx, incidentID, callIDs, noteAr)
}

func (s *store) Incidents(ctx context.Context, p IncidentsParams) (rows []database.Incident, totalCount int, err error) {
	db := database.FromCtx(ctx)

	offset, perPage := p.Pagination.OffsetPerPage(100)
	dbParam := database.ListIncidentsPParams{
		Start:     p.Start.PGTypeTSTZ(),
		End:       p.End.PGTypeTSTZ(),
		Direction: p.Direction.DirString(common.DirAsc),
		Offset:    offset,
		PerPage:   perPage,
	}

	var count int64
	txErr := db.InTx(ctx, func(db database.Store) error {
		var err error
		count, err = db.ListIncidentsCount(ctx, dbParam.Start, dbParam.End)
		if err != nil {
			return err
		}

		rows, err = db.ListIncidentsP(ctx, dbParam)
		return err
	}, pgx.TxOptions{})
	if txErr != nil {
		return nil, 0, txErr
	}

	return rows, int(count), err
}

func (s *store) Incident(ctx context.Context, id uuid.UUID) (database.Incident, error) {
	return database.FromCtx(ctx).GetIncident(ctx, id)
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

func (s *store) UpdateIncident(ctx context.Context, id uuid.UUID, p UpdateIncidentParams) (database.Incident, error) {
	db := database.FromCtx(ctx)

	return db.UpdateIncident(ctx, p.toDBUIP(id))
}

func (s *store) DeleteIncident(ctx context.Context, id uuid.UUID) error {
	return database.FromCtx(ctx).DeleteIncident(ctx, id)
}
