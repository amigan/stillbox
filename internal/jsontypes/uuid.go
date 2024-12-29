package jsontypes

import (
	"github.com/google/uuid"
)

type UUID uuid.UUID
type UUIDs []UUID

func (u *UUIDs) UUIDs() []uuid.UUID {
	r := make([]uuid.UUID, 0, len(*u))

	for _, v := range *u {
		r = append(r, v.UUID())
	}

	return r
}

func (u UUID) UUID() uuid.UUID {
	return uuid.UUID(u)
}

func (u *UUID) MarshalJSON() ([]byte, error) {
	return []byte(`"` + u.UUID().String() + `"`), nil
}

func (u *UUID) UnmarshalJSON(b []byte) error {
    id, err := uuid.Parse(string(b[:]))
    if err != nil {
            return err
    }
    *u = UUID(id)
    return nil
}

func (u *UUID) UnmarshalText(t []byte) error {
	var gu uuid.UUID
	err := gu.UnmarshalText(t)
	if err != nil {
		return err
	}
	*u = UUID(gu)

	return nil
}
