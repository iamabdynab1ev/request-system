package entities 

    import (
		"request-system/pkg/types"
	)

	type Role struct {
		ID          int       `json:"id"`
		Name        string    `json:"name"`
		Description string    `json:"description"`
		StatusID    int       `json:"status_id"`
		
		types.BaseEntity  
		types.SoftDelete 
	}