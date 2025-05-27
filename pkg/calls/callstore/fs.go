package callstore

import (
	"context"
	"fmt"
	"net/url"

	"dynatron.me/x/stillbox/pkg/calls"
	"dynatron.me/x/stillbox/pkg/config"
)

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
