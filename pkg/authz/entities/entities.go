package entities

import (
	"context"
	"fmt"
	"net/http"

	"github.com/el-mike/restrict/v2"
	"github.com/google/uuid"
)

const (
	RoleUser        = "User"
	RoleSubmitter   = "Submitter"
	RoleAdmin       = "Admin"
	RoleSystem      = "System"
	RolePublic      = "Public"
	RoleShareGuest  = "ShareGuest"
	RoleTranscriber = "Transcriber"

	ResourceCall      = "Call"
	ResourceIncident  = "Incident"
	ResourceTalkgroup = "Talkgroup"
	ResourceAlert     = "Alert"
	ResourceShare     = "Share"
	ResourceAPIKey    = "APIKey"
	ResourceCallStats = "CallStats"
	ResourceSetting   = "Setting"
	ResourceUser      = "User"

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

func CtxWithSubject(ctx context.Context, sub Subject) context.Context {
	return context.WithValue(ctx, SubjectCtxKey, sub)
}

func CtxWithServiceSubject(ctx context.Context, name string) context.Context {
	return CtxWithSubject(ctx, &SystemServiceSubject{Name: name})
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

type CallSubject struct {
	CallID uuid.UUID
}

func (s *CallSubject) GetRoles() []string {
	return []string{RoleTranscriber}
}

func (s *CallSubject) GetName() string {
	return "TRANSCRIBER:" + s.CallID.String()
}

func (s *CallSubject) String() string {
	return s.GetName()
}

func IsSystemService(sub Subject) bool {
	_, is := sub.(*SystemServiceSubject)

	return is
}
