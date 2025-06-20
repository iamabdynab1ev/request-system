package types

type Pagination struct {
	TotalCount uint64 `json:"total_count"`
	Page       uint64 `json:"page"`
	Limit      uint64 `json:"limit"`
}
