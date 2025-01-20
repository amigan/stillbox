package users

import (
	"dynatron.me/x/stillbox/pkg/rbac"
)

type Public struct {
	RemoteAddr string
}

func (s *Public) GetRoles() []string {
	return []string{rbac.RolePublic}
}
