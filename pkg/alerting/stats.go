package alerting

import (
	_ "embed"
	"html/template"
	"net/http"
	"time"

	"dynatron.me/x/stillbox/pkg/config"
	"dynatron.me/x/stillbox/pkg/database"
	"dynatron.me/x/stillbox/pkg/talkgroups"

	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/internal/trending"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
)

//go:embed stats.html
var statsTemplateFile string

var (
	statTmpl = template.Must(template.New("stats").Funcs(common.FuncMap).Parse(statsTemplateFile))
)

type stats interface {
	PrivateRoutes(chi.Router)
}

func (as *alerter) PrivateRoutes(r chi.Router) {
	r.Get("/tgstats", as.tgStatsHandler)
	r.Post("/tgstats", as.simulateHandler)
	r.Get("/testnotify", as.testNotifyHandler)
}

func (as *noopAlerter) PrivateRoutes(r chi.Router) {}

func (as *alerter) tgStatsHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	db := database.FromCtx(ctx)

	tgs, err := db.GetTalkgroupsWithLearnedByPackedIDs(ctx, as.packedScoredTGs())
	if err != nil {
		log.Error().Err(err).Msg("stats TG get failed")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	tgMap := make(map[talkgroups.ID]database.GetTalkgroupsWithLearnedByPackedIDsRow, len(tgs))
	for _, t := range tgs {
		tgMap[talkgroups.ID{System: uint32(t.System.ID), Talkgroup: uint32(t.Talkgroup.Tgid)}] = t
	}

	renderData := struct {
		TGs        map[talkgroups.ID]database.GetTalkgroupsWithLearnedByPackedIDsRow
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
