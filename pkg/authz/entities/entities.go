package entities

import (
	"context"
	"fmt"
	"net/http"

	"github.com/el-mike/restrict/v2"
)

const (
	RoleUser        = "User"
	RoleSubmitter   = "Submitter"
	RoleAdmin       = "Admin"
	RoleSystem      = "System"
	RolePublic      = "Public"
	RoleShareGuest  = "ShareGuest"
	RoleTranscriber = "Transcriber"
	RoleSharer      = "Sharer"

	ResourceCall       = "Call"
	ResourceIncident   = "Incident"
	ResourceTalkgroup  = "Talkgroup"
	ResourceSystem     = "System" // P25 system
	ResourceAlert      = "Alert"
	ResourceShare      = "Share"
	ResourceAPIKey     = "APIKey"
	ResourceCallStats  = "CallStats"
	ResourceSetting    = "Setting"
	ResourceUser       = "User"
	ResourceNexus      = "Nexus"
	ResourcePushSub    = "PushSubscription"
	ResourceWebPushSub = "WebPushSubscription"

	ActionRead             = "read"
	ActionReadPrivileged   = "readPrivileged"
	ActionCreate           = "create"
	ActionUpdate           = "update"
	ActionDelete           = "delete"
	ActionShare            = "share"
	ActionUpdatePrivileged = "updatePrivileged"
	ActionTranscribe       = "updateTranscription"
	ActionSimulate         = "simulate"
	ActionTestNotify       = "testNotify"
	ActionMoveCallAudio    = "moveCallAudio"
	ActionConnect          = "connect"

	ScopeSubmit     = "submit"
	ScopeTranscribe = "transcribe"
)

func SubjectFrom(ctx context.Context) Subject {
	sub, ok := ctx.Value(SubjectCtxKey).(Subject)
	if !ok {
		panic("no subject in context")
	}

	return sub
}

type Subject interface {
	fmt.Stringer
	restrict.Subject
	GetName() string
}

type SubjectWrapper interface {
	UnwrapSubject() Subject
}

func CtxWithSubject(ctx context.Context, sub Subject) context.Context {
	return context.WithValue(ctx, SubjectCtxKey, sub)
}

func CtxWithServiceSubject(ctx context.Context, name string) context.Context {
	return CtxWithSubject(ctx, &SystemServiceSubject{Name: name})
}

// HasRole returns whether the subject has any of the passed roles.
func HasRole(sub Subject, roles ...string) bool {
	r := sub.GetRoles()
	m := make(map[string]struct{}, len(r))
	for _, r := range r {
		m[r] = struct{}{}
	}

	for _, role := range roles {
		_, has := m[role]
		if has {
			return true
		}
	}

	return false
}

type subjectContextKey string

const SubjectCtxKey subjectContextKey = "sub"

type Resource interface {
	restrict.Resource
}

type PublicSubject struct {
	RemoteAddr string
}

func (s *PublicSubject) GetName() string {
	return "PUBLIC:" + s.RemoteAddr
}

func (s *PublicSubject) String() string {
	return s.GetName()
}

func (s *PublicSubject) GetRoles() []string {
	return []string{RolePublic}
}

func NewPublicSubject(r *http.Request) *PublicSubject {
	return &PublicSubject{RemoteAddr: r.RemoteAddr}
}

type LocalAdminSubject struct {
}

func (s *LocalAdminSubject) GetName() string {
	return "LOCALADMIN"
}

func (s *LocalAdminSubject) String() string {
	return s.GetName()
}

func (s *LocalAdminSubject) GetRoles() []string {
	return []string{RoleAdmin}
}

func NewLocalAdminSubject() *LocalAdminSubject {
	return new(LocalAdminSubject)
}

type SystemServiceSubject struct {
	Name string
}

func (s *SystemServiceSubject) GetName() string {
	return "SYSTEM:" + s.Name
}

func (s *SystemServiceSubject) String() string {
	return s.GetName()
}

func (s *SystemServiceSubject) GetRoles() []string {
	return []string{RoleSystem}
}

type APIKeySubject struct {
	Subject

	scopes []string
	roles  []string
}

func (s *APIKeySubject) UnwrapSubject() Subject {
	return s.Subject
}

var apiKeyScopes = map[string]string{
	ScopeSubmit:     RoleSubmitter,
	ScopeTranscribe: RoleTranscriber,
}

// ValidateScopes validates that all scopes exist. If any are invalid, it returns false.
func ValidateScopes(scopes []string) bool {
	for _, s := range scopes {
		if _, hasScope := apiKeyScopes[s]; !hasScope {
			return false
		}
	}

	return true
}

func (s *APIKeySubject) GetRoles() []string {
	if s.scopes != nil && s.roles == nil {
		s.roles = make([]string, 0, len(s.scopes))
		for _, sc := range s.scopes {
			if role, has := apiKeyScopes[sc]; has {
				s.roles = append(s.roles, role)
			}
		}
	}

	return s.roles
}

func NewAPIKeySubject(user Subject, scope ...string) *APIKeySubject {
	return &APIKeySubject{Subject: user, scopes: scope}
}

func IsSystemService(sub Subject) bool {
	_, is := sub.(*SystemServiceSubject)

	return is
}
