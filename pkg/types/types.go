package types

type Filter struct {
	Search         string                 `json:"search"`
	Sort           map[string]interface{} `json:"sort"`
	Filter         map[string]interface{} `json:"filter"`
	Limit          int                    `json:"limit"`
	Offset         int                    `json:"offset"`
	WithPagination bool                   `json:"withPagination"`
}

// http://localhost:8080/clients?search=Khujand&sort[created_at]=desc&filter[status_id]=1&filter[branch_id]=1&limit=10&offset=0&withPagination=true

// var allowedFields = []string{
// 	"status_id",
// 	"branch_id",
// }
