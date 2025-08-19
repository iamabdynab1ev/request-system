package types

// Filter represents query parameters for filtering and pagination.
type Filter struct {
	Search         string                 `json:"search,omitempty"`
	Sort           map[string]string      `json:"sort,omitempty"`
	Filter         map[string]interface{} `json:"filter,omitempty"`
	Limit          int                    `json:"limit"`
	Offset         int                    `json:"offset"`
	Page           int                    `json:"page"`
	WithPagination bool                   `json:"with_pagination"`
}

// Pagination represents pagination metadata.
type Pagination struct {
	TotalCount uint64 `json:"total_count"`
	Page       int    `json:"page"`
	Limit      int    `json:"limit"`
	TotalPages int    `json:"total_pages"`
}

// http://localhost:8080/clients?search=Khujand&sort[created_at]=desc&filter[status_id]=1&filter[branch_id]=1,2,4&limit=10&offset=0&withPagination=true

// var allowedFields = []string{
// 	"status_id",
// 	"branch_id",
// }
