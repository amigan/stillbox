// Package services avoids having to wrap contexts so much for multiple services.
package services

import (
	"context"
	"sync"
)

type Services interface {
	Set(key, val any)
	Value(key any) any
}

type services struct {
	sync.RWMutex
	m map[any]any
}

func New() Services {
	return &services{
		m: make(map[any]any),
	}
}

func CtxWith(ctx context.Context, svc Services) context.Context {
	return context.WithValue(ctx, ServiceKey, svc)
}

func (s *services) Value(key any) any {
	s.RLock()
	defer s.RUnlock()

	return s.m[key]
}

func (s *services) Set(key, val any) {
	s.Lock()
	defer s.Unlock()

	s.m[key] = val
}

type serviceKey string

const ServiceKey serviceKey = "service"

func WithValue(ctx context.Context, key, val any) context.Context {
	v := ctx.Value(ServiceKey)
	if v == nil {
		return context.WithValue(ctx, key, val)
	}

	sv := v.(Services)
	sv.Set(key, val)

	return ctx
}

func Value(ctx context.Context, key any) any {
	sv := ctx.Value(ServiceKey)
	if sv == nil {
		return ctx.Value(key)
	}

	return sv.(Services).Value(key)
}
