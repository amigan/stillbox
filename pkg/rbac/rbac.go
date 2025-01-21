package rbac

import (
	"context"
	"errors"

	"github.com/el-mike/restrict/v2"
	"github.com/el-mike/restrict/v2/adapters"
)

const (
	RoleUser       = "User"
	RoleSubmitter  = "Submitter"
	RoleAdmin      = "Admin"
	RoleSystem     = "System"
	RolePublic     = "Public"
	RoleShareGuest = "ShareGuest"

	ResourceCall      = "Call"
	ResourceIncident  = "Incident"
	ResourceTalkgroup = "Talkgroup"
	ResourceAlert     = "Alert"
	ResourceShare     = "Share"
	ResourceAPIKey    = "APIKey"

	ActionRead   = "read"
	ActionCreate = "create"
	ActionUpdate = "update"
	ActionDelete = "delete"
	ActionShare  = "share"

	PresetUpdateOwn       = "updateOwn"
	PresetDeleteOwn       = "deleteOwn"
	PresetReadShared      = "readShared"
	PresetReadSharedInMap = "readSharedInMap"
	PresetShareOwn        = "shareOwn"

	PresetUpdateSubmitter = "updateSubmitter"
	PresetDeleteSubmitter = "deleteSubmitter"
	PresetShareSubmitter  = "shareSubmitter"
)

var (
	ErrBadSubject = errors.New("bad subject in token")
)

type subjectContextKey string

const SubjectCtxKey subjectContextKey = "sub"

func CtxWithSubject(ctx context.Context, sub Subject) context.Context {
	return context.WithValue(ctx, SubjectCtxKey, sub)
}

func ErrAccessDenied(err error) *restrict.AccessDeniedError {
	if accessErr, ok := err.(*restrict.AccessDeniedError); ok {
		return accessErr
	}

	return nil
}

func SubjectFrom(ctx context.Context) Subject {
	sub, ok := ctx.Value(SubjectCtxKey).(Subject)
	if ok {
		return sub
	}

	return new(PublicSubject)
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

type Subject interface {
	restrict.Subject
	GetName() string
}

type Resource interface {
	restrict.Resource
}

type RBAC interface {
	Check(ctx context.Context, res restrict.Resource, opts ...CheckOption) (Subject, error)
}

type rbac struct {
	policy *restrict.PolicyManager
	access *restrict.AccessManager
}

func New() (*rbac, error) {
	adapter := adapters.NewInMemoryAdapter(policy)
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
func Check(ctx context.Context, res restrict.Resource, opts ...CheckOption) (Subject, error) {
	return FromCtx(ctx).Check(ctx, res, opts...)
}

func (r *rbac) Check(ctx context.Context, res restrict.Resource, opts ...CheckOption) (Subject, error) {
	sub := SubjectFrom(ctx)
	o := checkOptions{}

	for _, opt := range opts {
		opt(&o)
	}

	req := &restrict.AccessRequest{
		Subject:  sub,
		Resource: res,
		Actions:  o.actions,
		Context:  o.context,
	}

	return sub, r.access.Authorize(req)
}

type PublicSubject struct {
	RemoteAddr string
}

func (s *PublicSubject) GetName() string {
	return "PUBLIC:" + s.RemoteAddr
}

func (s *PublicSubject) GetRoles() []string {
	return []string{RolePublic}
}

type SystemServiceSubject struct {
	Name string
}

func (s *SystemServiceSubject) GetName() string {
	return "SYSTEM:" + s.Name
}

func (s *SystemServiceSubject) GetRoles() []string {
	return []string{RoleSystem}
}
