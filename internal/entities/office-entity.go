package entities
import (
	"time"
	"request-system/pkg/types"
)
type Office struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	Address     string    `json:"address"`
	OpenDate    time.Time `json:"open_date"`
	BranchID    int       `json:"branch_id"`
	StatusID    int       `json:"status_id"`

	types.BaseEntity	
}


