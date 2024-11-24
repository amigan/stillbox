package common

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
