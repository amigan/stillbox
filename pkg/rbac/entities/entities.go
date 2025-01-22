package entities

import (
	"context"

	"github.com/el-mike/restrict/v2"
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
)

func SubjectFrom(ctx context.Context) Subject {
	sub, ok := ctx.Value(SubjectCtxKey).(Subject)
	if ok {
		return sub
	}

	return new(PublicSubject)
}

type Subject interface {
	restrict.Subject
	GetName() string
}

func CtxWithSubject(ctx context.Context, sub Subject) context.Context {
	return context.WithValue(ctx, SubjectCtxKey, sub)
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
