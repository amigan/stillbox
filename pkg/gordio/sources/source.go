package sources

import (
	"context"
	"dynatron.me/x/stillbox/pkg/gordio/calls"

	"github.com/go-chi/chi/v5"
)

type Source interface {
	SourceType() string
}

type sourceInstance struct {
	Source
	Name string
}

type Sources []sourceInstance

func (s *Sources) Register(name string, src Source) {
	*s = append(*s, sourceInstance{
		Name:   name,
		Source: src,
	})
}

func (s *Sources) InstallPublicRoutes(r chi.Router) {
	for _, si := range *s {
		if rs, ok := si.Source.(PublicRouteSource); ok {
			rs.InstallPublicRoutes(r)
		}
	}
}

type Ingestor interface {
	Ingest(context.Context, *calls.Call)
}

type PublicRouteSource interface {
	InstallPublicRoutes(chi.Router)
}
