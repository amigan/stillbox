package users

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"dynatron.me/x/stillbox/pkg/database"
	"dynatron.me/x/stillbox/pkg/rbac"
	"dynatron.me/x/stillbox/pkg/rbac/entities"
)

type UserID int

func (u *UserID) Int32Ptr() *int32 {
	if u == nil {
		return nil
	}

	i := int32(*u)

	return &i
}

func (u UserID) Int() int {
	return int(u)
}

func (u UserID) IsValid() bool {
	return u > 0
}

func From(ctx context.Context) (*User, error) {
	sub := entities.SubjectFrom(ctx)
	return FromSubject(sub)
}

func UserCheck(ctx context.Context, rsc entities.Resource, actions string) (*User, error) {
	acts := strings.Split(actions, "+")
	subj, err := rbac.FromCtx(ctx).Check(ctx, rsc, rbac.WithActions(acts...))
	if err != nil {
		return nil, err
	}

	return FromSubject(subj)
}

func FromSubject(sub entities.Subject) (*User, error) {
	if sub == nil {
		return nil, rbac.ErrBadSubject
	}

	user, isUser := sub.(*User)
	if !isUser || user == nil || !user.ID.IsValid() {
		return nil, rbac.ErrBadSubject
	}

	return user, nil
}

type User struct {
	ID            UserID
	Username      string
	Password      string
	Email         string
	IsAdmin       bool
	LastLoginAt   *time.Time
	LastLoginFrom string
	Prefs         json.RawMessage
}

func (u *User) GetName() string {
	return u.Username
}

func (u *User) String() string {
	return "USER:" + u.GetName()
}

func (u *User) GetRoles() []string {
	r := make([]string, 1, 2)

	r[0] = entities.RoleUser

	if u.IsAdmin {
		r = append(r, entities.RoleAdmin)
	}

	return r
}

func fromDBUser(dbu database.User) *User {
	return &User{
		ID:       UserID(dbu.ID),
		Username: dbu.Username,
		Password: dbu.Password,
		Email:    dbu.Email,
		IsAdmin:  dbu.IsAdmin,
		Prefs:    dbu.Prefs,
	}
}
