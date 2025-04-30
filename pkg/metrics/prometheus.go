package metrics

import (
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"dynatron.me/x/stillbox/internal/acl"
	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/pkg/config"
	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"
)

// Metrics is the interface to the metrics subsystem.
type Metrics interface {
	// Register registers the provided struct under the provided subsystem name.
	// A MetricStruct is a struct containing members of the various prometheus metric types
	// including Counter, Gauge, Histogram, and Summary.
	// Struct tags:
	// "help" - the metric's help string
	// "buckets" - a list of bucket values for Histogram metrics
	Register(subsys string, ms MetricStruct)

	// Handler returns the metrics endpoint handler.
	Handler() http.Handler

	// InstallRoute installs the metrics endpoint handler in the provided chi router.
	InstallRoute(chi.Router)
}

type MetricStruct any

type metrics struct {
	reg *prometheus.Registry
	acl *acl.IP
	cfg config.Metrics
}

// NewMetrics creates a new Metrics.
func NewMetrics(cfg config.Metrics) (*metrics, error) {
	acl, err := cfg.ACL.IPACL()
	if err != nil {
		return nil, fmt.Errorf("metrics ACL: %w", err)
	}

	m := &metrics{
		reg: prometheus.NewRegistry(),
		acl: acl,
		cfg: cfg,
	}

	m.reg.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	m.reg.MustRegister(collectors.NewGoCollector())

	return m, nil
}

// shamelessly lifted from github.com/kelseyhightower/envconfig
var gatherRegexp = regexp.MustCompile("([^A-Z]+|[A-Z]+[^A-Z]+|[A-Z]+)")
var acronymRegexp = regexp.MustCompile("([A-Z]+)([A-Z][^A-Z]+)")

func (m *metrics) Register(subsys string, ms MetricStruct) {
	v := reflect.ValueOf(ms)

	for v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		panic("must be struct")
	}

	vt := v.Type()

	for fi := range v.NumField() {
		fv := v.Field(fi)
		ft := vt.Field(fi)
		if !ft.Type.Implements(reflect.TypeFor[prometheus.Collector]()) {
			continue
		}

		var metricName string
		words := gatherRegexp.FindAllStringSubmatch(ft.Name, -1)
		if len(words) > 0 {
			var name []string
			for _, words := range words {
				if m := acronymRegexp.FindStringSubmatch(words[0]); len(m) == 3 {
					name = append(name, strings.ToLower(m[1]), strings.ToLower(m[2]))
				} else {
					name = append(name, strings.ToLower(words[0]))
				}
			}

			metricName = strings.Join(name, "_")
		}

		if metricName == "" {
			panic("metric name empty")
		}

		help := ft.Tag.Get("help")

		var buckets []float64
		if buckStr, has := ft.Tag.Lookup("buckets"); has {
			ba := strings.Split(buckStr, ",")
			buckets = make([]float64, 0, len(ba))
			for _, b := range ba {
				bf, err := strconv.ParseFloat(b, 64)
				if err != nil {
					panic("bad bucket value")
				}

				buckets = append(buckets, bf)
			}
		}

		opts := prometheus.Opts{
			Namespace: common.AppName,
			Subsystem: subsys,
			Name:      metricName,
			Help:      help,
		}

		switch ft.Type.Name() {
		case "Counter":
			fv.Set(reflect.ValueOf(prometheus.NewCounter(prometheus.CounterOpts(opts))))
		case "Gauge":
			fv.Set(reflect.ValueOf(prometheus.NewGauge(prometheus.GaugeOpts(opts))))
		case "Histogram":
			fv.Set(reflect.ValueOf(prometheus.NewHistogram(prometheus.HistogramOpts{
				Namespace: common.AppName,
				Subsystem: subsys,
				Name:      metricName,
				Help:      help,
				Buckets:   buckets,
			})))
		case "Summary":
			fv.Set(reflect.ValueOf(prometheus.NewSummary(prometheus.SummaryOpts{
				Namespace: common.AppName,
				Subsystem: subsys,
				Name:      metricName,
				Help:      help,
			})))
		default:
			panic("unsupported metric type " + ft.Type.Name())
		}

		m.reg.Register(fv.Interface().(prometheus.Collector))
	}
}

func (m *metrics) InstallRoute(r chi.Router) {
	if m.cfg.Enabled {
		if m.cfg.Path == "" {
			m.cfg.Path = "/metrics"
		}

		r.Handle(m.cfg.Path, m.Handler())
	}
}

func (m *metrics) Handler() http.Handler {
	hnd := promhttp.HandlerFor(m.reg, promhttp.HandlerOpts{
		Registry: m.reg,
	})

	checkReq := m.acl.Allowed
	if m.cfg.Password != "" && m.cfg.Username != "" {
		checkReq = func(r *http.Request) error {
			err := m.acl.Allowed(r)
			if err != nil {
				return err
			}

			un, pw, ok := r.BasicAuth()
			if !ok || !(un == m.cfg.Username && pw == m.cfg.Password) {
				return errors.New("unauthorized")
			}

			return nil
		}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := checkReq(r)
		if err != nil {
			log.Error().Err(err).Str("remote", r.RemoteAddr).Msg("metrics")
			http.Error(w, "access denied", http.StatusUnauthorized)
			return
		}

		hnd.ServeHTTP(w, r)
	})
}
