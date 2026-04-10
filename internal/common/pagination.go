package common

import "errors"

var (
	ErrPageOutOfRange = errors.New("requested page out of range")
	ErrBadOrder       = errors.New("invalid order")
	ErrBadDirection   = errors.New("invalid direction")
)

// Pagination describes a request for a particular page and records per page.
type Pagination struct {
	Page    *int `json:"page"`
	PerPage *int `json:"perPage"`
}

// OffsetPerPage computes a sane offset and records per page based on p and perPageDefault.
func (p Pagination) OffsetPerPage(perPageDefault int) (offset int32, perPage int32) {
	page := int32(DefaultIfNilOrZero(p.Page, 1))
	perPage = int32(DefaultIfNilOrZero(p.PerPage, perPageDefault))

	offset = (page - 1) * perPage

	return
}

type SortDirection string

const (
	DirAsc  SortDirection = "asc"
	DirDesc SortDirection = "desc"
)

// DirString returns the direction or a default.
func (t *SortDirection) DirString(def SortDirection) string {
	if t == nil {
		return string(def)
	}

	return string(*t)
}

// IsValid returns whether t is valid.
func (t *SortDirection) IsValid() bool {
	if t == nil {
		return true
	}

	switch *t {
	case DirAsc, DirDesc:
		return true
	}

	return false

}
