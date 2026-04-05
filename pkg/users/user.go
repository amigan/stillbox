package users

import (
	"context"
	"encoding/json"
	"errors"
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

	if sw, isWrapper := sub.(entities.SubjectWrapper); isWrapper {
		sub = sw.UnwrapSubject()
	}

	user, isUser := sub.(*User)
	if !isUser || user == nil || !user.ID.IsValid() {
		return nil, authz.ErrBadSubject
	}

	return user, nil
}

// Password is a password field; it can be hashed or plain.
type Password interface {
	// Hash returns the hash of the password.
	Hash() (string, error)

	// String returns the raw field as it is (plain or hashed)
	String() string
}

type plainPassword string

func NewPlainPassword(pass string) (pp plainPassword, err error) {
	pass = strings.TrimSpace(pass)
	if len(pass) < 5 { // sanity check; callers may impose stricter requirements
		return pp, errors.New("bad password")
	}

	return pp, nil
}

func (p plainPassword) Hash() (string, error) {
	return HashPassword(string(p))
}

func (p plainPassword) String() string {
	return string(p)
}

type HashedPassword string

func (hp HashedPassword) Hash() (string, error) {
	return string(hp), nil
}

func (hp HashedPassword) String() string {
	return string(hp)
}

// A User is a user record.
type User struct {
	ID            UserID
	Username      string
	Password      Password
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
	return &User{
		ID:            UserID(dbu.ID),
		Username:      dbu.Username,
		Password:      HashedPassword(dbu.Password),
		Email:         dbu.Email,
		RealName:      dbu.RealName,
		Roles:         dbu.Roles,
		Prefs:         dbu.Prefs,
		DisabledAt:    (*jsontypes.Time)(dbu.DisabledAt),
		LastLoginAt:   (*jsontypes.Time)(dbu.LastLoginAt),
		LastLoginFrom: dbu.LastLoginFrom,
		PasswordSetAt: dbu.PasswordSetAt,
	}
}
