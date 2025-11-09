package users

import (
	"context"
	"encoding/json"
	"net/netip"
	"slices"
	"strings"
	"time"

	"dynatron.me/x/stillbox/internal/jsontypes"
	"dynatron.me/x/stillbox/pkg/authz"
	"dynatron.me/x/stillbox/pkg/authz/entities"
	"dynatron.me/x/stillbox/pkg/database"
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
	subj, err := authz.FromCtx(ctx).Check(ctx, rsc, authz.WithActions(acts...))
	if err != nil {
		return nil, err
	}

	return FromSubject(subj)
}

func FromSubject(sub entities.Subject) (*User, error) {
	if sub == nil {
		return nil, authz.ErrBadSubject
	}

	user, isUser := sub.(*User)
	if !isUser || user == nil || !user.ID.IsValid() {
		return nil, authz.ErrBadSubject
	}

	return user, nil
}

// A User is a user record.
type User struct {
	ID            UserID
	Username      string
	Password      string
	Email         string
	RealName      *string
	Roles         []string
	DisabledAt    *jsontypes.Time
	LastLoginAt   *jsontypes.Time
	LastLoginFrom *netip.Addr
	Prefs         json.RawMessage
	PasswordSetAt time.Time
}

func (u *User) HasRole(role string) bool {
	return slices.Contains(u.Roles, role)
}

func (*User) GetResourceName() string {
	return entities.ResourceUser
}

func (u *User) GetName() string {
	return u.Username
}

func (u *User) String() string {
	return "USER:" + u.GetName()
}

func (u *User) GetRoles() []string {
	return append(u.Roles, entities.RoleUser)
}

func (u *User) Mask() *User {
	return &User{
		ID:       u.ID,
		Username: u.Username,
		RealName: u.RealName,
	}
}

func FromDBUser(dbu database.User) *User {
	var lastLoginAt, disabledAt *jsontypes.Time
	if dbu.LastLoginAt.Valid {
		lastLoginAt = (*jsontypes.Time)(&dbu.LastLoginAt.Time)
	}

	if dbu.DisabledAt.Valid {
		disabledAt = (*jsontypes.Time)(&dbu.DisabledAt.Time)
	}

	return &User{
		ID:            UserID(dbu.ID),
		Username:      dbu.Username,
		Password:      dbu.Password,
		Email:         dbu.Email,
		RealName:      dbu.RealName,
		Roles:         dbu.Roles,
		Prefs:         dbu.Prefs,
		DisabledAt:    disabledAt,
		LastLoginAt:   lastLoginAt,
		LastLoginFrom: dbu.LastLoginFrom,
		PasswordSetAt: dbu.PasswordSetAt.Time,
	}
}
