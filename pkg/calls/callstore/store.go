package callstore

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/internal/jsontypes"
	"dynatron.me/x/stillbox/internal/trending"

	"dynatron.me/x/stillbox/pkg/authz"
	"dynatron.me/x/stillbox/pkg/authz/entities"
	"dynatron.me/x/stillbox/pkg/calls"
	"dynatron.me/x/stillbox/pkg/config"
	"dynatron.me/x/stillbox/pkg/database"
	"dynatron.me/x/stillbox/pkg/database/partman"
	"dynatron.me/x/stillbox/pkg/metrics"
	"dynatron.me/x/stillbox/pkg/services"
	"dynatron.me/x/stillbox/pkg/talkgroups"
	"dynatron.me/x/stillbox/pkg/talkgroups/tgstore"
	"dynatron.me/x/stillbox/pkg/users"

	"github.com/google/uuid"
	"github.com/hashicorp/go-multierror"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog/log"
)

type Store interface {
	// AddCall adds a call to the database.
	AddCall(ctx context.Context, call *calls.Call) error

	// DeleteCall deletes a call.
	Delete(ctx context.Context, id uuid.UUID) error

	// CallAudio returns a CallAudio struct.
	// If io.EOF is returned, request headers and the end of the response have already been sent if the request and suitable backing connection are available.
	CallAudio(ctx context.Context, id uuid.UUID, opts ...CallAudioOption) (*calls.CallAudio, error)

	// Call returns the call's metadata.
	Call(ctx context.Context, id uuid.UUID) (*calls.Call, error)

	// CompleteCalls returns calls with audio and metadata.
	CompleteCalls(ctx context.Context, ids jsontypes.UUIDs) ([]*calls.Call, error)

	// Calls gets paginated Calls.
	Calls(ctx context.Context, p ListCallsParams) (calls []database.ListCallsPRow, totalCount int, err error)

	// CallStats gets call stats by interval.
	CallStats(ctx context.Context, interval calls.StatsInterval, start, end jsontypes.Time) (*calls.Stats, error)

	// BackfillTrending backfills call statistics into a trending scorer.
	BackfillTrending(ctx context.Context, scorer *trending.Scorer[talkgroups.ID], stepClock func(time.Time), since, until time.Time) (count int, err error)

	// UpdateTranscription updates a call's transcription.
	UpdateTranscription(ctx context.Context, id uuid.UUID, text *string) (*calls.CallTranscription, error)

	// TranscriptContext gets a talkgroup's last recent calls with length greater than threshold and since lookback ago.
	TranscriptContext(ctx context.Context, tg talkgroups.ID, count uint, threshold jsontypes.Duration, lookback jsontypes.Duration) ([]database.GetTranscriptsContextRow, error)

	// MoveCallAudio moves call audio from one backend to another.
	MoveCallAudio(ctx context.Context, par MoveCallParams) (numRows int64, err error)

	// BlobPath generates the audio path for FS and S3 backends. audioPath is the real path into
	// the store, while audioRef will contain a leading `_/` if partitioning is enabled.
	BlobPath(call *calls.CallAudio) (audioPath, audioRef string)

	// GoGC starts the audio garbage collector thread.
	GoGC(ctx context.Context)

	// DoGC runs an audio garbage collection cycle.
	DoGC(context.Context, chan error)

	partman.PartmanCallAudioManager
	PartMan() partman.PartitionManager
}

type store struct {
	db            database.Store
	audioBackends *audioBackends
	partman       partman.PartitionManager
	now           NowFunc

	moveInProgress sync.Mutex
}

func (s *store) PartMan() partman.PartitionManager {
	return s.partman
}

type StoreOption func(*store)
type NowFunc func() time.Time

func WithNow(nf NowFunc) StoreOption {
	return func(s *store) {
		s.now = nf
	}
}

func NewStore(ctx context.Context, db database.Store, tgc tgstore.FilterCache, met metrics.Metrics, callStorage config.CallStorage, partConfig config.Partition, opts ...StoreOption) (*store, error) {
	st := &store{
		db:  db,
		now: time.Now,
	}

	for _, opt := range opts {
		opt(st)
	}

	be, err := st.MakeBackends(ctx, tgc, met, callStorage)
	if err != nil {
		return nil, fmt.Errorf("call storage: %w", err)
	}

	st.audioBackends = be

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	errCh := make(chan error)
	go func() {
		for {
			select {
			case err := <-errCh:
				log.Error().Err(err).Msg("call audio prune fail")
			case <-ctx.Done():
				return
			}
		}
	}()
	gcCount, gcAttempted, err := st.audioBackends.journal.GC(ctx, database.GetRefJournalParams{}, errCh)
	if err != nil {
		return nil, err
	}

	if gcAttempted > 0 {
		log.Info().Int64("count", gcCount).Int64("attempted", gcAttempted).Msg("call audio garbage collected")
	}

	if partConfig.Enabled {
		st.partman, err = partman.NewPartitionManager(db, st, partConfig)
		if err != nil {
			return nil, err
		}

		err = st.partman.Check(ctx, st.now().UTC())
		if err != nil {
			return nil, err
		}
	}

	return st, nil
}

type storeCtxKey string

const StoreCtxKey storeCtxKey = "store"

func CtxWithStore(ctx context.Context, s Store) context.Context {
	return services.WithValue(ctx, StoreCtxKey, s)
}

func FromCtx(ctx context.Context) Store {
	s, ok := services.Value(ctx, StoreCtxKey).(Store)
	if !ok {
		panic("no call store in context")
	}

	return s
}

func audioMimeFromString(s string) database.NullAudioMIME {
	if s == "" {
		return database.NullAudioMIME{}
	}

	return database.NullAudioMIME{
		AudioMIME: database.AudioMIME(s),
		Valid:     true,
	}
}

func toAddCallParams(call *calls.Call, audioRef AudioRefJSON, audioBlob []byte) database.AddCallParams {
	return database.AddCallParams{
		ID:          call.ID,
		Submitter:   call.Submitter.Int32Ptr(),
		System:      call.System,
		Talkgroup:   call.Talkgroup,
		CallDate:    call.DateTime,
		AudioName:   common.NilIfZero(call.AudioName),
		AudioBlob:   audioBlob,
		AudioType:   audioMimeFromString(call.AudioType),
		AudioRef:    audioRef,
		Duration:    call.Duration.MsInt32Ptr(),
		Frequency:   call.Frequency,
		Frequencies: call.Frequencies,
		Patches:     call.Patches,
		TalkerAlias: call.TalkerAlias,
		TGLabel:     call.TalkgroupLabel,
		TGAlphaTag:  call.TGAlphaTag,
		TGGroup:     call.TalkgroupGroup,
		Source:      call.Source,
	}
}

func (s *store) AddCall(ctx context.Context, call *calls.Call) (err error) {
	_, err = authz.Check(ctx, call, authz.WithActions(entities.ActionCreate))
	if err != nil {
		return err
	}

	rt := s.audioBackends.RefTracker()
	rt.Reset()
	defer func() {
		s.audioBackends.PutRefTracker(rt)
	}()

	blob := call.Audio
	var refJ AudioRefJSON
	audioRef, err := s.audioBackends.Store(ctx, call)
	if err != nil {
		return fmt.Errorf("add call: %w", err)
	}

	if audioRef != nil { // was stored in a backend
		blob = nil
		rt.Created(audioRef.Backend, AbsoluteRef(audioRef.Ref.Ref(s.partman, call.DateTime)))

		refMap := map[string]AudioRef{
			audioRef.Backend.Name: audioRef.Ref,
		}

		refJ, err = json.Marshal(refMap)
		if err != nil {
			rbe := rt.Rollback(ctx)
			if rbe != nil {
				err = multierror.Append(err, rbe)
			}

			return err
		}
	}

	defer func() {
		if err != nil {
			rbe := rt.Rollback(ctx)
			if rbe != nil {
				err = multierror.Append(err, rbe)
			}
		} else {
			ce := rt.Commit(ctx)
			// doesn't truly matter here since there will never be anything committable (only creates)
			// but log it anyay
			if ce != nil {
				log.Error().Err(ce).Msg("audioRefTracker commit failed")
			}
		}
	}()

	params := toAddCallParams(call, refJ, blob)
	db := database.FromCtx(ctx)
	tgs := tgstore.FromCtx(ctx)

	err = db.InTx(ctx, func(tx database.Store) error {
		err := tx.AddCall(ctx, params)
		if err != nil {
			return fmt.Errorf("add call: %w", err)
		}

		return nil
	}, pgx.TxOptions{})

	if err != nil && database.IsTGConstraintViolation(err) {
		return db.InTx(ctx, func(tx database.Store) error {
			_, err := tgs.LearnTG(ctx, call)
			if err != nil {
				return fmt.Errorf("learn tg: %w", err)
			}

			err = tx.AddCall(ctx, params)
			if err != nil {
				return fmt.Errorf("learn tg retry: %w", err)
			}

			return nil
		}, pgx.TxOptions{})
	}

	return err
}

type ZeroCopyResponseWriter interface {
	http.ResponseWriter
	http.Flusher
	io.ReaderFrom
}

type CallAudioOptions struct {
	isDownload  bool         // Content-Disposition: attachment
	resolveBlob bool         // don't return a URL, return the blob
	audioRefOut AudioRefList // if not nil, fill with passed backend json
	zcrw        ZeroCopyResponseWriter
}

type CallAudioOption func(*CallAudioOptions)

func WithDownloadDisposition(isDownload bool) CallAudioOption {
	return func(o *CallAudioOptions) {
		o.isDownload = isDownload
	}
}

func WithResponseWriter(w http.ResponseWriter) CallAudioOption {
	return func(o *CallAudioOptions) {
		zc, isZc := w.(ZeroCopyResponseWriter)
		if isZc {
			o.zcrw = zc
		}
	}
}

func WithResolveBlob() CallAudioOption {
	return func(o *CallAudioOptions) {
		o.resolveBlob = true
	}
}

func WithAudioRefOut(arl AudioRefList) CallAudioOption {
	return func(o *CallAudioOptions) {
		o.audioRefOut = arl
	}
}

func (s *store) CallAudio(ctx context.Context, id uuid.UUID, opts ...CallAudioOption) (*calls.CallAudio, error) {
	_, err := authz.Check(ctx, &calls.Call{ID: id}, authz.WithActions(entities.ActionRead))
	if err != nil {
		return nil, err
	}

	options := CallAudioOptions{}
	for _, opt := range opts {
		opt(&options)
	}

	db := database.FromCtx(ctx)

	dbCall, err := db.GetCallAudioByID(ctx, id)
	if err != nil {
		return nil, err
	}

	audioMime := func(a database.NullAudioMIME) *string {
		if a.Valid {
			return common.PtrTo(string(a.AudioMIME))
		}

		return nil
	}

	call := &calls.CallAudio{
		ID:        id,
		CallDate:  jsontypes.Time(dbCall.CallDate),
		AudioName: dbCall.AudioName,
		AudioType: audioMime(dbCall.AudioType),
		AudioBlob: dbCall.AudioBlob,
	}

	blob := dbCall.AudioBlob
	if ref := dbCall.AudioRef; blob == nil && ref != nil {
		err := s.audioBackends.CallAudio(ctx, call, ref, &options)
		if err != nil {
			// even if it's io.EOF due to sendfile (ReadFrom) being used, just pass it up
			return nil, err
		}
	}

	return call, nil
}

func (s *store) CompleteCalls(ctx context.Context, ids jsontypes.UUIDs) ([]*calls.Call, error) {
	_, err := authz.Check(ctx, authz.UseResource(entities.ResourceCall), authz.WithActions(entities.ActionRead))
	if err != nil {
		return nil, err
	}

	db := database.FromCtx(ctx)

	c, err := db.GetCalls(ctx, ids.UUIDs())
	if err != nil {
		return nil, err
	}

	cs := make([]*calls.Call, 0, len(c))
	for _, dbc := range c {
		c := dbc.Call
		var sub *users.UserID
		if c.Submitter != nil {
			sub = common.PtrTo(users.UserID(*c.Submitter))
		}

		// set this up in case we need to resolve from backend
		callAud := calls.CallAudio{
			ID:        c.ID,
			CallDate:  jsontypes.Time(c.CallDate),
			AudioBlob: c.AudioBlob,
		}

		if c.AudioBlob == nil {
			// will fill in callAud.AudioBlob
			err := s.audioBackends.CallAudio(ctx, &callAud, c.AudioRef, &CallAudioOptions{resolveBlob: true})
			if err != nil {
				return nil, err
			}
		}

		cs = append(cs, &calls.Call{
			ID:             c.ID,
			Submitter:      sub,
			System:         c.System,
			Talkgroup:      c.Talkgroup,
			DateTime:       c.CallDate,
			Audio:          callAud.AudioBlob,
			AudioName:      common.ZeroIfNil(c.AudioName),
			AudioType:      string(c.AudioType.AudioMIME),
			Duration:       calls.CallDuration(time.Duration(common.ZeroIfNil(c.Duration)) * time.Millisecond),
			Frequency:      c.Frequency,
			Frequencies:    c.Frequencies,
			Patches:        c.Patches,
			TalkerAlias:    c.TalkerAlias,
			TalkgroupLabel: c.TGLabel,
			TalkgroupGroup: c.TGGroup,
			TGAlphaTag:     c.TGAlphaTag,
			Transcript:     c.Transcript,
		})
	}

	return cs, nil
}

func (s *store) Call(ctx context.Context, id uuid.UUID) (*calls.Call, error) {
	_, err := authz.Check(ctx, &calls.Call{ID: id}, authz.WithActions(entities.ActionRead))
	if err != nil {
		return nil, err
	}

	db := database.FromCtx(ctx)

	c, err := db.GetCall(ctx, id)
	if err != nil {
		return nil, err
	}

	var sub *users.UserID
	if c.Submitter != nil {
		sub = common.PtrTo(users.UserID(*c.Submitter))
	}

	return &calls.Call{
		ID:             c.ID,
		Submitter:      sub,
		System:         c.System,
		Talkgroup:      c.Talkgroup,
		DateTime:       c.CallDate,
		AudioName:      common.ZeroIfNil(c.AudioName),
		AudioType:      string(c.AudioType.AudioMIME),
		Duration:       calls.CallDuration(time.Duration(common.ZeroIfNil(c.Duration)) * time.Millisecond),
		Frequency:      c.Frequency,
		Frequencies:    c.Frequencies,
		Patches:        c.Patches,
		TalkerAlias:    c.TalkerAlias,
		TalkgroupLabel: c.TGLabel,
		TalkgroupGroup: c.TGGroup,
		TGAlphaTag:     c.TGAlphaTag,
		Transcript:     c.Transcript,
	}, nil
}

type CallsParams struct {
	Start            *jsontypes.Time   `json:"start,omitempty" desc:"timestamp start" flag:"start s"`
	End              *jsontypes.Time   `json:"end,omitempty" desc:"timestamp end" flag:"end e"`
	TagsAny          []string          `json:"tagsAny,omitempty" desc:"talkgroup tags contain any"`
	TagsNot          []string          `json:"tagsNot,omitempty" desc:"talkgroup tags doesn't contain"`
	TGFilter         *jsontypes.String `json:"tgFilter,omitempty" desc:"talkgroup name filter" flag:"tg-filter t"`
	SourceFilter     *string           `json:"sourceFilter,omitempty" desc:"source contains"`
	AtLeastSeconds   *float32          `json:"atLeastSeconds,omitempty" desc:"call length at least seconds" flag:"length l"`
	UnknownTG        bool              `json:"unknownTG,omitzero" desc:"talkgroup is unknown" flag:"unknown-tg u"`
	TranscriptSearch *string           `json:"transcriptSearch,omitempty" desc:"transcript contains" flag:"transcript-search T"`
}

type ListCallsParams struct {
	common.Pagination
	Direction *common.SortDirection `json:"dir"`

	CallsParams
}

func toPGNumericMilliseconds[T int | uint | int64 | uint64 | float32 | float64](f *T) (n pgtype.Numeric) {
	if f == nil {
		return
	}

	if err := n.Scan(fmt.Sprint((*f) * 1000)); err != nil {
		panic("cannot convert to PG numeric")
	}

	return
}

func (s *store) Calls(ctx context.Context, p ListCallsParams) (rows []database.ListCallsPRow, totalCount int, err error) {
	_, err = authz.Check(ctx, authz.UseResource(entities.ResourceCall), authz.WithActions(entities.ActionRead))
	if err != nil {
		return nil, 0, err
	}

	db := database.FromCtx(ctx)

	offset, perPage := p.Pagination.OffsetPerPage(100)
	par := database.ListCallsPParams{
		Start:            (*time.Time)(p.Start),
		End:              (*time.Time)(p.End),
		TagsAny:          p.TagsAny,
		TagsNot:          p.TagsNot,
		Offset:           offset,
		PerPage:          perPage,
		Direction:        p.Direction.DirString(common.DirAsc),
		TGFilter:         p.TGFilter.StringPtr(),
		SourceFilter:     p.SourceFilter,
		UnknownTG:        p.UnknownTG,
		TranscriptSearch: p.TranscriptSearch,
	}

	par.LongerThan = toPGNumericMilliseconds(p.AtLeastSeconds)

	var count int64
	txErr := db.InTx(ctx, func(db database.Store) error {
		var err error
		count, err = db.ListCallsCount(ctx, database.ListCallsCountParams{
			Start:            par.Start,
			End:              par.End,
			TagsAny:          par.TagsAny,
			TagsNot:          par.TagsNot,
			TGFilter:         par.TGFilter,
			SourceFilter:     p.SourceFilter,
			LongerThan:       par.LongerThan,
			UnknownTG:        par.UnknownTG,
			TranscriptSearch: par.TranscriptSearch,
		})
		if err != nil {
			return err
		}

		if offset > int32(count) {
			return common.ErrPageOutOfRange
		}

		rows, err = db.ListCallsP(ctx, par)
		return err
	}, pgx.TxOptions{})
	if txErr != nil {
		return nil, 0, txErr
	}

	return rows, int(count), err
}

func (s *store) Delete(ctx context.Context, id uuid.UUID) error {
	callOwn, err := s.getCallOwner(ctx, id)
	if err != nil {
		return err
	}

	_, err = authz.Check(ctx, &callOwn, authz.WithActions(entities.ActionDelete))
	if err != nil {
		return err
	}

	return database.FromCtx(ctx).DeleteCall(ctx, id)
}

func (s *store) getCallOwner(ctx context.Context, id uuid.UUID) (calls.Call, error) {
	subInt, err := database.FromCtx(ctx).GetCallSubmitter(ctx, id)

	var sub *users.UserID

	if subInt != nil {
		sub = common.PtrTo(users.UserID(*subInt))
	}
	return calls.Call{ID: id, Submitter: sub}, err
}

func (s *store) CallStats(ctx context.Context, interval calls.StatsInterval, start, end jsontypes.Time) (*calls.Stats, error) {
	if !interval.IsValid() {
		return nil, calls.ErrInvalidInterval
	}

	cs := &calls.Stats{
		Interval: interval,
	}

	_, err := authz.Check(ctx, cs, authz.WithActions(entities.ActionRead))
	if err != nil {
		return nil, err
	}

	db := database.FromCtx(ctx)

	dbs, err := db.GetCallStatsByInterval(ctx, string(interval), (*time.Time)(&start), (*time.Time)(&end))
	if err != nil {
		return nil, err
	}

	cs.Stats = make([]calls.Stat, 0, len(dbs))
	for _, st := range dbs {
		cs.Stats = append(cs.Stats, calls.Stat{
			Count: st.Count,
			Time:  jsontypes.Time(st.Date),
		})
	}

	return cs, nil
}

func (s *store) UpdateTranscription(ctx context.Context, id uuid.UUID, text *string) (*calls.CallTranscription, error) {
	c, err := s.getCallOwner(ctx, id)
	if err != nil {
		return nil, err
	}

	_, err = authz.Check(ctx, &c, authz.WithActions(entities.ActionTranscribe))
	if err != nil {
		return nil, err
	}

	sts, err := database.FromCtx(ctx).SetCallTranscript(ctx, id, text)
	if err != nil {
		return nil, err
	}

	tsc := &calls.CallTranscription{
		TG:         talkgroups.TG(sts.System, sts.Talkgroup),
		Patches:    sts.Patches,
		Transcript: text,
	}

	return tsc, nil
}

func (s *store) BackfillTrending(ctx context.Context, scorer *trending.Scorer[talkgroups.ID], stepClock func(time.Time), since, until time.Time) (count int, err error) {
	// We can do this through stats grants
	_, err = authz.Check(ctx, &calls.Stats{}, authz.WithActions(entities.ActionRead))
	if err != nil {
		return 0, err
	}

	db := database.FromCtx(ctx)
	const backfillStatsQuery = `SELECT system, talkgroup, call_date FROM calls WHERE call_date > $1 AND call_date < $2 ORDER BY call_date ASC`

	rows, err := db.DB().Query(ctx, backfillStatsQuery, since, until)
	if err != nil {
		return count, err
	}
	defer rows.Close()

	for rows.Next() {
		var tg talkgroups.ID
		var callDate time.Time
		if err := rows.Scan(&tg.System, &tg.Talkgroup, &callDate); err != nil {
			return count, err
		}
		scorer.AddEvent(tg, callDate)
		if stepClock != nil { // step the simulator if it is active
			stepClock(callDate)
		}
		count++
	}

	if err := rows.Err(); err != nil {
		return count, err
	}

	return count, nil
}

func (s *store) TranscriptContext(ctx context.Context, tg talkgroups.ID, count uint, threshold jsontypes.Duration, lookback jsontypes.Duration) ([]database.GetTranscriptsContextRow, error) {
	_, err := authz.Check(ctx, authz.UseResource(entities.ResourceCall), authz.WithActions(entities.ActionRead))
	if err != nil {
		return nil, err
	}

	db := database.FromCtx(ctx)
	return db.GetTranscriptsContext(ctx, database.GetTranscriptsContextParams{
		System:         int(tg.System),
		Talkgroup:      int(tg.Talkgroup),
		DurationMS:     common.PtrTo(int32(threshold.Duration().Milliseconds())),
		NumTranscripts: int32(count),
		Lookback:       lookback.PGInterval(),
	})
}
