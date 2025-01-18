package rbac

import (
	"context"
	"errors"
	"fmt"
	"reflect"

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

	PresetUpdateOwn  = "updateOwn"
	PresetDeleteOwn  = "deleteOwn"
	PresetReadShared = "readShared"

	PresetUpdateSubmitter = "updateSubmitter"
	PresetDeleteSubmitter = "deleteSubmitter"
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

var policy = &restrict.PolicyDefinition{
	Roles: restrict.Roles{
		RoleUser: {
			Description: "An authenticated user",
			Grants: restrict.GrantsMap{
				ResourceIncident: {
					&restrict.Permission{Action: ActionRead},
					&restrict.Permission{Action: ActionCreate},
					&restrict.Permission{Preset: PresetUpdateOwn},
					&restrict.Permission{Preset: PresetDeleteOwn},
				},
				ResourceCall: {
					&restrict.Permission{Action: ActionRead},
					&restrict.Permission{Action: ActionCreate},
					&restrict.Permission{Preset: PresetUpdateSubmitter},
					&restrict.Permission{Preset: PresetDeleteSubmitter},
				},
				ResourceTalkgroup: {
					&restrict.Permission{Action: ActionRead},
				},
				ResourceShare: {
					&restrict.Permission{Action: ActionRead},
					&restrict.Permission{Action: ActionCreate},
					&restrict.Permission{Preset: PresetUpdateOwn},
					&restrict.Permission{Preset: PresetDeleteOwn},
				},
			},
		},
		RoleSubmitter: {
			Description: "A role that can submit calls",
			Grants: restrict.GrantsMap{
				ResourceCall: {
					&restrict.Permission{Action: ActionCreate},
				},
				ResourceTalkgroup: {
					// for learning TGs
					&restrict.Permission{Action: ActionCreate},
					&restrict.Permission{Action: ActionUpdate},
				},
			},
		},
		RoleShareGuest: {
			Description: "Someone who has a valid share link",
			Grants: restrict.GrantsMap{
				ResourceCall: {
					&restrict.Permission{Preset: PresetReadShared},
				},
				ResourceIncident: {
					&restrict.Permission{Preset: PresetReadShared},
				},
				ResourceTalkgroup: {
					&restrict.Permission{Action: ActionRead},
				},
			},
		},
		RoleAdmin: {
			Parents: []string{RoleUser},
			Grants: restrict.GrantsMap{
				ResourceIncident: {
					&restrict.Permission{Action: ActionUpdate},
					&restrict.Permission{Action: ActionDelete},
				},
				ResourceCall: {
					&restrict.Permission{Action: ActionUpdate},
					&restrict.Permission{Action: ActionDelete},
				},
				ResourceTalkgroup: {
					&restrict.Permission{Action: ActionUpdate},
					&restrict.Permission{Action: ActionCreate},
					&restrict.Permission{Action: ActionDelete},
				},
			},
		},
		RoleSystem: {
			Parents: []string{RoleSystem},
		},
		RolePublic: {
			/*
				Grants: restrict.GrantsMap{
					ResourceShare: {
						&restrict.Permission{Action: ActionRead},
					},
				},
			*/
		},
	},
	PermissionPresets: restrict.PermissionPresets{
		PresetUpdateOwn: &restrict.Permission{
			Action: ActionUpdate,
			Conditions: restrict.Conditions{
				&restrict.EqualCondition{
					ID: "isOwner",
					Left: &restrict.ValueDescriptor{
						Source: restrict.ResourceField,
						Field:  "Owner",
					},
					Right: &restrict.ValueDescriptor{
						Source: restrict.SubjectField,
						Field:  "ID",
					},
				},
			},
		},
		PresetDeleteOwn: &restrict.Permission{
			Action: ActionDelete,
			Conditions: restrict.Conditions{
				&restrict.EqualCondition{
					ID: "isOwner",
					Left: &restrict.ValueDescriptor{
						Source: restrict.ResourceField,
						Field:  "Owner",
					},
					Right: &restrict.ValueDescriptor{
						Source: restrict.SubjectField,
						Field:  "ID",
					},
				},
			},
		},
		PresetUpdateSubmitter: &restrict.Permission{
			Action: ActionUpdate,
			Conditions: restrict.Conditions{
				&SubmitterEqualCondition{
					ID: "isSubmitter",
					Left: &restrict.ValueDescriptor{
						Source: restrict.ResourceField,
						Field:  "Submitter",
					},
					Right: &restrict.ValueDescriptor{
						Source: restrict.SubjectField,
						Field:  "ID",
					},
				},
			},
		},
		PresetDeleteSubmitter: &restrict.Permission{
			Action: ActionDelete,
			Conditions: restrict.Conditions{
				&SubmitterEqualCondition{
					ID: "isSubmitter",
					Left: &restrict.ValueDescriptor{
						Source: restrict.ResourceField,
						Field:  "Submitter",
					},
					Right: &restrict.ValueDescriptor{
						Source: restrict.SubjectField,
						Field:  "ID",
					},
				},
			},
		},
		PresetReadShared: &restrict.Permission{
			Action: ActionRead,
			Conditions: restrict.Conditions{
				&restrict.EqualCondition{
					ID: "isOwner",
					Left: &restrict.ValueDescriptor{
						Source: restrict.ContextField,
						Field:  "Owner",
					},
					Right: &restrict.ValueDescriptor{
						Source: restrict.SubjectField,
						Field:  "ID",
					},
				},
			},
		},
	},
}

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

type ShareLinkGuest struct {
	ShareID string
}

func (s *ShareLinkGuest) GetName() string {
	return "SHARE:" + s.ShareID
}

func (s *ShareLinkGuest) GetRoles() []string {
	return []string{RoleShareGuest}
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

const (
	SubmitterEqualConditionType = "SUBMITTER_EQUAL"
)

type SubmitterEqualCondition struct {
	ID    string                    `json:"name,omitempty" yaml:"name,omitempty"`
	Left  *restrict.ValueDescriptor `json:"left" yaml:"left"`
	Right *restrict.ValueDescriptor `json:"right" yaml:"right"`
}

func (s *SubmitterEqualCondition) Type() string {
	return SubmitterEqualConditionType
}

func (c *SubmitterEqualCondition) Check(r *restrict.AccessRequest) error {
	left, err := c.Left.GetValue(r)
	if err != nil {
		return err
	}

	right, err := c.Right.GetValue(r)
	if err != nil {
		return err
	}

	lVal := reflect.ValueOf(left)
	rVal := reflect.ValueOf(right)

	// deref Left. this is the difference between us and EqualCondition
	for lVal.Kind() == reflect.Pointer {
		lVal = lVal.Elem()
	}

	if !lVal.IsValid() || !reflect.DeepEqual(rVal.Interface(), lVal.Interface()) {
		return restrict.NewConditionNotSatisfiedError(c, r, fmt.Errorf("values \"%v\" and \"%v\" are not equal", left, right))
	}

	return nil
}

func SubmitterEqualConditionFactory() restrict.Condition {
	return new(SubmitterEqualCondition)
}

func init() {
	restrict.RegisterConditionFactory(SubmitterEqualConditionType, SubmitterEqualConditionFactory)
}
