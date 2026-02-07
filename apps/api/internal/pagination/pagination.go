package pagination

import (
	"math"

	"gorm.io/gorm"
)

// PageRequest holds pagination parameters parsed from query strings.
type PageRequest struct {
	Page     int `form:"page" binding:"omitempty,min=1"`
	PageSize int `form:"page_size" binding:"omitempty,min=1,max=100"`
}

// Defaults fills in default values when page or page_size are not provided.
func (p *PageRequest) Defaults() {
	if p.Page == 0 {
		p.Page = 1
	}
	if p.PageSize == 0 {
		p.PageSize = 20
	}
}

// Offset returns the SQL OFFSET for the current page.
func (p *PageRequest) Offset() int {
	return (p.Page - 1) * p.PageSize
}

// PageResponse wraps a paginated list of items with metadata.
type PageResponse[T any] struct {
	Data       []T   `json:"data"`
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	TotalItems int64 `json:"total_items"`
	TotalPages int   `json:"total_pages"`
}

// NewPageResponse creates a PageResponse from the given data and total count.
func NewPageResponse[T any](data []T, page, pageSize int, totalItems int64) PageResponse[T] {
	totalPages := int(math.Ceil(float64(totalItems) / float64(pageSize)))
	if data == nil {
		data = []T{}
	}
	return PageResponse[T]{
		Data:       data,
		Page:       page,
		PageSize:   pageSize,
		TotalItems: totalItems,
		TotalPages: totalPages,
	}
}

// Paginate returns a GORM scope that applies OFFSET and LIMIT for the given page request.
func Paginate(req PageRequest) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Offset(req.Offset()).Limit(req.PageSize)
	}
}
