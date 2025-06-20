package dto

type Pagination struct {
	TotalCount  uint64 `json:"total_count"`
	Limit       uint64 `json:"limit"`
	Offset      uint64 `json:"offset"`
	CurrentPage uint64 `json:"current_page,omitempty"`
	TotalPages  uint64 `json:"total_pages,omitempty"`
}

type PaginatedSuccessResponse[T any] struct {
	Status     bool        `json:"status"`
	Message    string      `json:"message"`
	Data       []T         `json:"data"`
	Pagination *Pagination `json:"pagination,omitempty"`
}
