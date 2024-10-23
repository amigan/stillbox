package alerting

import (
	_ "embed"
	"fmt"
	"html/template"
	"net/http"
	"time"

	"dynatron.me/x/stillbox/pkg/calls"
	"dynatron.me/x/stillbox/pkg/gordio/database"

	"dynatron.me/x/stillbox/internal/trending"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
)

//go:embed stats.html
var statsTemplateFile string

var (
	funcMap = template.FuncMap{
		"f": func(v float64) string {
			return fmt.Sprintf("%.4f", v)
		},
	}
	statTmpl = template.Must(template.New("stats").Funcs(funcMap).Parse(statsTemplateFile))
)

func (as *Alerter) PrivateRoutes(r chi.Router) {
	r.Get("/tgstats", as.tgStats)
}

func (as *Alerter) tgStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	db := database.FromCtx(ctx)

	packed := make([]int64, 0, len(as.scores))
	for _, s := range as.scores {
		packed = append(packed, s.ID.Pack())
	}

	tgs, err := db.GetTalkgroupsByPackedIDs(ctx, packed)
	if err != nil {
		log.Error().Err(err).Msg("stats TG get failed")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	tgMap := make(map[calls.Talkgroup]database.GetTalkgroupsByPackedIDsRow, len(tgs))
	for _, t := range tgs {
		tgMap[calls.Talkgroup{System: uint32(t.SystemID), Talkgroup: uint32(t.ID)}] = t
	}

	renderData := struct {
		TGs       map[calls.Talkgroup]database.GetTalkgroupsByPackedIDsRow
		Scores    trending.Scores[calls.Talkgroup]
		LastScore time.Time
	}{
		TGs:       tgMap,
		Scores:    as.scores,
		LastScore: as.lastScore,
	}

	w.WriteHeader(http.StatusOK)
	err = statTmpl.Execute(w, renderData)
	if err != nil {
		log.Error().Err(err).Msg("stat template exec")
	}
}
