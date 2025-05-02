package alerting

import (
	_ "embed"
	"html/template"
	"net/http"
	"time"

	"dynatron.me/x/stillbox/pkg/authz"
	"dynatron.me/x/stillbox/pkg/authz/entities"
	"dynatron.me/x/stillbox/pkg/config"
	"dynatron.me/x/stillbox/pkg/database"
	"dynatron.me/x/stillbox/pkg/talkgroups"

	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/internal/trending"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/csrf"
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
	_, err := authz.Check(ctx, authz.UseResource(entities.ResourceAlert), authz.WithActions(entities.ActionRead))
	if authz.IsErrAccessDenied(err) != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	if err != nil {
		log.Error().Err(err).Msg("rbac check failed")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	db := database.FromCtx(ctx)

	tgt := as.scoredTGsTuple()
	tgs, err := db.GetTalkgroupsWithLearnedBySysTGID(ctx, tgt)
	if err != nil {
		log.Error().Err(err).Msg("stats TG get failed")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	tgMap := make(map[talkgroups.ID]database.GetTalkgroupsRow, len(tgs))
	for _, t := range tgs {
		tgMap[talkgroups.ID{System: uint32(t.System.ID), Talkgroup: uint32(t.Talkgroup.TGID)}] = t
	}

	renderData := struct {
		TGs        map[talkgroups.ID]database.GetTalkgroupsRow
		Scores     trending.Scores[talkgroups.ID]
		LastScore  time.Time
		Simulation *Simulation
		Config     config.Alerting
		CSRFToken  string
	}{
		TGs:        tgMap,
		Scores:     as.scores,
		LastScore:  as.lastScore,
		Config:     as.cfg,
		Simulation: as.sim,
		CSRFToken:  csrf.Token(r),
	}

	w.WriteHeader(http.StatusOK)
	print(renderData.CSRFToken)
	err = statTmpl.Execute(w, renderData)
	if err != nil {
		log.Error().Err(err).Msg("stat template exec")
	}
}
