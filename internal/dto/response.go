package dto

type PaginatedResponse[T any] struct {
	List       []T              `json:"list"`
	Pagination PaginationObject `json:"pagination"`
}
type PaginationObject struct {
	TotalCount uint64 `json:"total_count"`
	Page       uint64 `json:"page"`
	Limit      uint64 `json:"limit"`
}
