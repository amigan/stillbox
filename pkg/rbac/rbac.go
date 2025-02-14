package rbac

import (
	"context"
	"errors"

	"dynatron.me/x/stillbox/pkg/rbac/entities"

	"github.com/el-mike/restrict/v2"
	"github.com/el-mike/restrict/v2/adapters"
)

var (
	ErrBadSubject = errors.New("bad subject in token")
	ErrAccessDenied = errors.New("access denied")
)

func IsErrAccessDenied(err error) error {
	if accessErr, ok := err.(*restrict.AccessDeniedError); ok {
		return accessErr
	}

	if err == ErrAccessDenied {
		return err
	}

	return nil
}

type rbacCtxKey string

const RBACCtxKey rbacCtxKey = "rbac"

func FromCtx(ctx context.Context) RBAC {
	rbac, ok := ctx.Value(RBACCtxKey).(RBAC)
	if !ok {
		panic("no RBAC in context")
	}

	return rbac
}

func CtxWithRBAC(ctx context.Context, rbac RBAC) context.Context {
	return context.WithValue(ctx, RBACCtxKey, rbac)
}

var (
	ErrNotAuthorized = errors.New("not authorized")
)

type checkOptions struct {
	actions []string
	context restrict.Context
}

type CheckOption func(*checkOptions)

func WithActions(actions ...string) CheckOption {
	return func(o *checkOptions) {
		o.actions = append(o.actions, actions...)
	}
}

func WithContext(ctx restrict.Context) CheckOption {
	return func(o *checkOptions) {
		o.context = ctx
	}
}

func UseResource(rsc string) restrict.Resource {
	return restrict.UseResource(rsc)
}

type RBAC interface {
	Check(ctx context.Context, res restrict.Resource, opts ...CheckOption) (entities.Subject, error)
}

type rbac struct {
	policy *restrict.PolicyManager
	access *restrict.AccessManager
}

func New(pol *restrict.PolicyDefinition) (*rbac, error) {
	adapter := adapters.NewInMemoryAdapter(pol)
	polMan, err := restrict.NewPolicyManager(adapter, true)
	if err != nil {
		return nil, err
	}

	accMan := restrict.NewAccessManager(polMan)
	return &rbac{
		policy: polMan,
		access: accMan,
	}, nil
}

// Check is a convenience function to pull the RBAC instance out of ctx and Check.
func Check(ctx context.Context, res restrict.Resource, opts ...CheckOption) (entities.Subject, error) {
	return FromCtx(ctx).Check(ctx, res, opts...)
}

func (r *rbac) Check(ctx context.Context, res restrict.Resource, opts ...CheckOption) (entities.Subject, error) {
	sub := entities.SubjectFrom(ctx)
	o := checkOptions{}

	for _, opt := range opts {
		opt(&o)
	}

	if o.context == nil {
		o.context = make(restrict.Context)
	}

	o.context["ctx"] = ctx

	req := &restrict.AccessRequest{
		Subject:  sub,
		Resource: res,
		Actions:  o.actions,
		Context:  o.context,
	}

	authRes := r.access.Authorize(req)

	return sub, authRes
}
