package callstore

import (
	"context"
	"fmt"
	"net/url"

	"dynatron.me/x/stillbox/pkg/calls"
	"dynatron.me/x/stillbox/pkg/calls/filter"
	"dynatron.me/x/stillbox/pkg/config"
	"dynatron.me/x/stillbox/pkg/metrics"
	"dynatron.me/x/stillbox/pkg/talkgroups/tgstore"
	"github.com/goccy/go-json"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog/log"
)

type AudioRef any

type AudioBackend interface {
	StoreCall(context.Context, *calls.Call) (AudioRef, error)
	GetCall(ctx context.Context, audioName *string, audioRef AudioRef, resolveBlob bool) (blob []byte, audioURL *url.URL, err error)

	Type() string
}

type AudioBackends interface {
	// Store tries all backends and stores the call if any match.
	// If the call was stored, audioRef will be non-nil.
	// If the call was not stored, but not due to an error, AudioRef will be nil along with err.
	Store(ctx context.Context, call *calls.Call) (AudioRefJSON, error)

	// CallAudio gets the call audio from the backend and location specified by audioRef.
	// It returns either a non-nil blob or url, but never both.
	// resolveBlob disables URL generation.
	CallAudio(ctx context.Context, audioName *string, audioRef AudioRefJSON, resolveBlob bool) (blob []byte, audioURL *url.URL, err error)
}

type audioBackends struct {
	storeList []string // in config order

	backends map[string]*audioStorageBackend
	metrics  audioStorageMetrics
}

type audioStorageBackend struct {
	Name    string
	Filter  *filter.Filter
	OnError config.StorageDisposition
	AudioBackend
	met audioStorageMetrics
}

type audioStorageMetrics struct {
	TotalStores  *prometheus.CounterVec `help:"Total call stores." labels:"backend,type"`
	FailedStores *prometheus.CounterVec `help:"Failed call storage attempts by backend." labels:"backend,type"`
}

type fsBackend struct {
}

func (*fsBackend) Type() string { return "fs" }

func (fsb *fsBackend) GetCall(_ context.Context, _ *string, _ AudioRef, _ bool) ([]byte, *url.URL, error) {
	return nil, nil, fmt.Errorf("FS backend not implemented")
}

func (fsb *fsBackend) StoreCall(_ context.Context, _ *calls.Call) (AudioRef, error) {
	return nil, fmt.Errorf("FS backend not implemented")
}

func newFSbackend(cfg config.ConfigMap) (*fsBackend, error) {
	return nil, fmt.Errorf("FS backend not implemented")
}

var _ AudioBackends = (*audioBackends)(nil)

func (sb *audioBackends) CallAudio(ctx context.Context, audioName *string, audioRef AudioRefJSON, resolveBlob bool) (blob []byte, ur *url.URL, err error) {
	var refm map[string]AudioRef
	err = json.Unmarshal(audioRef, &refm)
	if err != nil {
		return
	}

	for backend, location := range refm {
		be, has := sb.backends[backend]
		if !has {
			err = fmt.Errorf("no such backend '%s'", backend)
			continue
		}
		blob, ur, err = be.GetCall(ctx, audioName, location, resolveBlob)
		if err == nil {
			break
		}
	}

	return
}

func (ab *audioBackends) Store(ctx context.Context, call *calls.Call) (arj AudioRefJSON, err error) {
	for _, beName := range ab.storeList {
		be, has := ab.backends[beName]
		if !has {
			// this should never happen
			return nil, fmt.Errorf("no such backend '%s'", beName)
		}

		res := be.Filter.Test(ctx, call)
		if !res {
			continue
		}

		var ref AudioRef
		ref, err = be.StoreCall(ctx, call)
		if err != nil {
			ab.metrics.FailedStores.WithLabelValues(beName, be.Type()).Inc()

			switch be.OnError {
			case config.OnErrorFail:
				return nil, fmt.Errorf("backend '%s': %w", beName, err)
			case config.OnErrorDB:
				log.Error().Str("callID", call.ID.String()).Err(err).Msg("failed to store audio, storing in DB")
				return nil, nil
			case config.OnErrorNextThenDB:
				log.Error().Str("callID", call.ID.String()).Err(err).Msg("failed to store audio, trying next then storing in DB")
				err = nil // so if nobody else stores, it stores in DB
				continue
			case config.OnErrorNextThenFail: // default
				log.Error().Str("callID", call.ID.String()).Err(err).Msg("failed to store audio, trying next")
				continue
			}
		} else if ref != nil {
			ab.metrics.TotalStores.WithLabelValues(beName, be.Type()).Inc()
		}

		refMap := map[string]AudioRef{
			beName: ref,
		}

		return json.Marshal(refMap)
	}

	return
}

func MakeBackends(ctx context.Context, fc tgstore.FilterCache, met metrics.Metrics, cfg []config.CallStorage) (*audioBackends, error) {
	ab := &audioBackends{
		storeList: make([]string, 0, len(cfg)),
		backends:  make(map[string]*audioStorageBackend, len(cfg)),
	}

	for _, cf := range cfg {
		if cf.Name == "" {
			return nil, fmt.Errorf("blank name invalid")
		}

		if _, exists := ab.backends[cf.Name]; exists {
			return nil, fmt.Errorf("backend with duplicate name '%s'", cf.Name)
		}

		var filt *filter.Filter
		var err error
		var be AudioBackend
		if cf.Filter != nil {
			filt, err = filter.FromMap(cf.Filter)
			if err != nil {
				return nil, fmt.Errorf("filter '%s': %w", cf.Name, err)
			}
		}

		switch cf.Backend {
		case "s3":
			be, err = newS3backend(ctx, cf.Config)
		case "fs":
			be, err = newFSbackend(cf.Config)
		default:
			return nil, fmt.Errorf("unknown backend '%s'", cf.Backend)
		}

		if err != nil {
			return nil, fmt.Errorf("backend '%s': %w", cf.Name, err)
		}

		ab.backends[cf.Name] = &audioStorageBackend{
			Name:         cf.Name,
			Filter:       filt,
			OnError:      cf.OnError,
			AudioBackend: be,
		}

		if !cf.ReadOnly { // readonly backends simply don't get added to the list for store calls
			ab.storeList = append(ab.storeList, cf.Name)
		}
	}

	met.Register("callaudio", &ab.metrics)

	return ab, nil
}
