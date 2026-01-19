//go:build integration

package callstore_test

// Tests for S3 audio backend.

import (
	"context"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/internal/testutil"
	"dynatron.me/x/stillbox/pkg/calls"
	"dynatron.me/x/stillbox/pkg/config"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/lifecycle"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const TestBucketName = "test"

type s3TestInstance struct {
	bucket string

	b2Mode        bool
	oldPrefixMode bool
	svr           *httptest.Server
	url           *url.URL
	cli           *minio.Client
	lc            lifecycle.Configuration
}

func (s *s3TestInstance) prefixFromRule(r lifecycle.Rule) string {
	if s.oldPrefixMode {
		return r.Prefix
	}

	return r.RuleFilter.Prefix
}

func (s *s3TestInstance) doLifecycle(ctx context.Context, t *testing.T) {
	objCh := make(chan minio.ObjectInfo)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		defer close(objCh)

		for _, r := range s.lc.Rules {
			if ctx.Err() != nil {
				return
			}

			for obj := range s.cli.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{Prefix: s.prefixFromRule(r), Recursive: true}) {
				if obj.Err != nil {
					t.Log(obj.Err)
				}

				objCh <- obj
			}
		}
	}()

	errCh := s.cli.RemoveObjects(ctx, s.bucket, objCh, minio.RemoveObjectsOptions{})
	for err := range errCh {
		t.Fatal(err)
	}
}

func (s *s3TestInstance) objMap(ctx context.Context) map[string]struct{} {
	res := make(map[string]struct{})
	for obj := range s.cli.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{Recursive: true}) {
		if obj.Err != nil {
			panic(obj.Err)
		}

		res[strip(obj.Key)] = struct{}{}
	}

	return res
}

func buildRuleMap(r []lifecycle.Rule) map[string]lifecycle.Rule {
	res := make(map[string]lifecycle.Rule)
	for _, rl := range r {
		res[rl.ID] = rl
	}

	return res
}

// lifecycleWrapper emulates lifecycle calls since fakes3 doesn't support them.
func (ti *s3TestInstance) lifecycleWrapper(hnd http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		qh := func(s string) bool {
			_, has := q[s]
			return has
		}

		tbn := "/" + ti.bucket + "/"

		switch {
		case r.Method == http.MethodGet && r.URL.Path == tbn && qh("lifecycle"):
			xe := xml.NewEncoder(w)
			err := xe.Encode(&ti.lc)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
			}
			return
		case r.Method == http.MethodPut && r.URL.Path == tbn && qh("lifecycle"):
			xd := xml.NewDecoder(r.Body)
			err := xd.Decode(&ti.lc)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
			}

			return
		case r.Method == http.MethodDelete && r.URL.Path == tbn && qh("lifecycle"):
			ti.lc.Rules = nil
			return
		}

		hnd.ServeHTTP(w, r)
	})
}

// strip removes the first path component.
func strip(s string) string {
	return s[strings.IndexRune(s, '/')+1:]
}

func (db *DBTestSuite) minCallDate(ctx context.Context) (time.Time, error) {
	minMaxRow := db.db.QueryRow(ctx, "SELECT MIN(call_date) FROM calls;")
	var minCallDate pgtype.Timestamptz
	err := minMaxRow.Scan(&minCallDate)
	if err != nil {
		return time.Time{}, err
	}

	if !minCallDate.Valid {
		panic("invalid date")
	}

	return minCallDate.Time, nil
}

func (db *DBTestSuite) setPruneAfters(ctx context.Context) error {
	_, err := db.db.Exec(ctx, "UPDATE audio_ref_journal SET prune_after = NOW() - INTERVAL '10' HOUR WHERE prune_after IS NOT NULL;")
	return err
}

func checkReferences(ctx context.Context, t *testing.T, s3 *s3TestInstance, dbt *DBTestSuite, noCallsBefore time.Time, oldMCD time.Time) {
	objs := s3.objMap(ctx)
	mcd, err := dbt.minCallDate(ctx)
	require.NoError(t, err)
	assert.True(t, mcd.After(noCallsBefore), "minCallDate is before no calls before")
	assert.NotEqual(t, oldMCD, mcd)

	objList := make([]string, 0, len(objs))
	for k := range objs {
		objList = append(objList, k)
	}

	var count int
	qry := dbt.db.QueryRow(ctx, "SELECT COUNT(*) FROM calls WHERE substring(audio_ref->>'s3' FROM 3) = ANY($1);", objList)
	err = qry.Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, len(objList), count, "calls in store that arent in DB")

	rows, err := dbt.db.Query(ctx, "SELECT substring(audio_ref->>'s3' FROM 3) FROM calls WHERE audio_ref IS NOT NULL;")
	require.NoError(t, err)
	defer rows.Close()

	for rows.Next() {
		var ref string
		err := rows.Scan(&ref)
		require.NoError(t, err)

		_, has := objs[ref]
		assert.True(t, has, "ref from DB doesn't exist")
	}
}

func newFakeS3(ctx context.Context, bucketName string, legacyPrefix bool) *s3TestInstance {
	s3backend := s3mem.New()
	faker := gofakes3.New(s3backend)

	ti := &s3TestInstance{
		bucket:        bucketName,
		oldPrefixMode: legacyPrefix,
	}

	ti.svr = httptest.NewServer(ti.lifecycleWrapper(faker.Server()))

	var err error
	ti.url, err = url.Parse(ti.svr.URL)
	if err != nil {
		panic(err)
	}
	ti.cli, err = minio.New(ti.url.Host, &minio.Options{})
	if err != nil {
		panic(err)
	}

	err = ti.cli.MakeBucket(ctx, ti.bucket, minio.MakeBucketOptions{})
	if err != nil {
		panic(err)
	}

	return ti
}

func (s *s3TestInstance) BackendConfig(b2mode bool, legacyPrefix bool) config.StorageBackendConfig {
	return config.StorageBackendConfig{
		Name:    "s3",
		Backend: "s3",
		OnError: config.OnErrorFail,
		Config: config.ConfigMap{
			"bucket":       TestBucketName,
			"endpoint":     s.url.Host,
			"isB2":         b2mode,
			"legacyPrefix": legacyPrefix,
		},
		Ingest: true,
	}
}

type s3testHook func(ctx context.Context, ti *s3TestInstance)

func TestS3Prune(t *testing.T) {
	ctx := fillCtxRbac(t, t.Context())

	tests := []struct {
		desc          string
		numCalls      int
		interval      common.Interval
		numPartitions int
		preProvision  int
		retain        int
		jitter        float32
		b2Mode        bool
		legacyPrefix  bool
	}{
		{
			desc:          "base",
			numCalls:      40,
			interval:      common.Daily,
			numPartitions: 3,
			preProvision:  5,
			retain:        1,
			jitter:        0.1,
		},
		{
			desc:          "daily jitter",
			numCalls:      400,
			interval:      common.Daily,
			numPartitions: 3,
			preProvision:  5,
			retain:        1,
			jitter:        0.7,
		},
		{
			desc:          "monthly",
			numCalls:      40,
			interval:      common.Monthly,
			numPartitions: 3,
			preProvision:  5,
			retain:        1,
			jitter:        0.1,
		},
		{
			desc:          "monthly b2",
			numCalls:      40,
			interval:      common.Monthly,
			numPartitions: 3,
			preProvision:  5,
			retain:        1,
			jitter:        0.1,
			b2Mode:        true,
		},
		{
			desc:          "monthly legacy prefix",
			numCalls:      40,
			interval:      common.Monthly,
			numPartitions: 3,
			preProvision:  5,
			retain:        1,
			jitter:        0.1,
			legacyPrefix:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			fs3 := newFakeS3(ctx, TestBucketName, tc.legacyPrefix)
			storeCfg := config.CallStorage{
				Backends: []config.StorageBackendConfig{
					fs3.BackendConfig(tc.b2Mode, tc.legacyPrefix),
				},
			}

			// base must be in the past
			baseTime := time.Now().Add(time.Duration(-1*tc.numPartitions) * tc.interval.Duration())
			curNow := baseTime

			now := func() time.Time {
				return curNow
			}
			st, ctx := NewDBTestSuite(ctx, t, storeCfg, testutil.CustomPartConfig(
				config.Partition{
					Enabled:      true,
					Interval:     string(tc.interval),
					Retain:       tc.retain,
					PreProvision: common.NilIfZero(tc.preProvision),
				},
			), testutil.WithNow(now))
			defer st.TearDownTest()

			for i, callDate := range testutil.StatCaller(t, baseTime, tc.numCalls, tc.numPartitions, tc.jitter, tc.interval) {
				err := st.store.AddCall(ctx, &calls.Call{
					ID:        uuid.New(),
					DateTime:  callDate,
					AudioType: "audio/mpeg",
					AudioName: callDate.Format("2006_01_02_15_04_05.999999") + ".mp3",
					Audio:     testutil.SmallMP3(),
					Talkgroup: 10101,
					System:    0x197,
				})
				require.NoError(t, err, "call index %d date %s", i, callDate.String())
			}
			mcd, err := st.minCallDate(ctx)
			require.NoError(t, err)

			curNow = time.Now()
			err = st.store.PartMan().Check(ctx, curNow)
			require.NoError(t, err)

			if tc.b2Mode {
				rm := buildRuleMap(fs3.lc.Rules)
				for k, r := range rm {
					if strings.HasPrefix(k, "sb_") && !strings.HasSuffix(k, "_marker") {
						_, has := rm[k+"_marker"]
						assert.True(t, has, "no b2 marker rule")
						assert.NotZero(t, r.NoncurrentVersionExpiration)
					}
				}
			}

			for _, r := range fs3.lc.Rules {
				if tc.legacyPrefix {
					assert.NotZero(t, r.Prefix)
				} else {
					assert.Zero(t, r.Prefix)
				}
			}

			fs3.doLifecycle(ctx, t)
			checkReferences(ctx, t, fs3, st, baseTime, mcd)

			require.NoError(t, st.setPruneAfters(ctx))

			errs := make([]error, 0)
			ec := make(chan error)
			go func() {
				for err := range ec {
					errs = append(errs, err)
				}
			}()
			st.store.DoGC(ctx, ec)
			assert.Len(t, errs, 0)

			assert.Len(t, fs3.lc.Rules, 0)
		})
	}
}
