package alerting

import (
	_ "embed"
	"errors"
	"html/template"
	"net/http"
	"time"

	"dynatron.me/x/stillbox/pkg/config"
	"dynatron.me/x/stillbox/pkg/database"
	"dynatron.me/x/stillbox/pkg/talkgroups"

	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/internal/jsontime"
	"dynatron.me/x/stillbox/internal/trending"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
)

//go:embed stats.html
var statsTemplateFile string

type stats interface {
	PrivateRoutes(chi.Router)
}

var (
	funcMap = template.FuncMap{
		"f": common.FmtFloat,
		"dict": func(values ...interface{}) (map[string]interface{}, error) {
			if len(values)%2 != 0 {
				return nil, errors.New("invalid dict call")
			}
			dict := make(map[string]interface{}, len(values)/2)
			for i := 0; i < len(values); i += 2 {
				key, ok := values[i].(string)
				if !ok {
					return nil, errors.New("dict keys must be strings")
				}
				dict[key] = values[i+1]
			}
			return dict, nil
		},
		"formTime": func(t jsontime.Time) string {
			return time.Time(t).Format("2006-01-02T15:04")
		},
		"ago": func(s string) (string, error) {
			d, err := time.ParseDuration(s)
			if err != nil {
				return "", err
			}
			return time.Now().Add(-d).Format("2006-01-02T15:04"), nil
		},
	}
	statTmpl = template.Must(template.New("stats").Funcs(funcMap).Parse(statsTemplateFile))
)

func (as *alerter) PrivateRoutes(r chi.Router) {
	r.Get("/tgstats", as.tgStatsHandler)
	r.Post("/tgstats", as.simulateHandler)
	r.Get("/testnotify", as.testNotifyHandler)
}

func (as *noopAlerter) PrivateRoutes(r chi.Router) {}

func (as *alerter) tgStatsHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	db := database.FromCtx(ctx)

	tgs, err := db.GetTalkgroupsByPackedIDs(ctx, as.packedScoredTGs())
	if err != nil {
		log.Error().Err(err).Msg("stats TG get failed")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	tgMap := make(map[talkgroups.ID]database.GetTalkgroupsByPackedIDsRow, len(tgs))
	for _, t := range tgs {
		tgMap[talkgroups.ID{System: uint32(t.System.ID), Talkgroup: uint32(t.Talkgroup.ID)}] = t
	}

	renderData := struct {
		TGs        map[talkgroups.ID]database.GetTalkgroupsByPackedIDsRow
		Scores     trending.Scores[talkgroups.ID]
		LastScore  time.Time
		Simulation *Simulation
		Config     config.Alerting
	}{
		TGs:        tgMap,
		Scores:     as.scores,
		LastScore:  as.lastScore,
		Config:     as.cfg,
		Simulation: as.sim,
	}

	w.WriteHeader(http.StatusOK)
	err = statTmpl.Execute(w, renderData)
	if err != nil {
		log.Error().Err(err).Msg("stat template exec")
	}
}
