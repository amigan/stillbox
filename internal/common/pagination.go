package common

import "errors"

var (
	ErrPageOutOfRange = errors.New("requested page out of range")
)

type Pagination struct {
	Page    *int `json:"page"`
	PerPage *int `json:"perPage"`
}

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

func (t *SortDirection) DirString(def SortDirection) string {
	if t == nil {
		return string(def)
	}

	return string(*t)
}

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
