package users

import (
	"dynatron.me/x/stillbox/pkg/rbac"
)

type ShareLinkGuest struct {
	ShareID string
}

func (s *ShareLinkGuest) GetRoles() []string {
	return []string{rbac.RoleShareGuest}
}

type Public struct {
	RemoteAddr string
}

func (s *Public) GetRoles() []string {
	return []string{rbac.RolePublic}
}
