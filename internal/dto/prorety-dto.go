package dto


type CreateProretyDTO struct {
	Icon string `json:"icon" validate:"required"`
	Name string `json:"name" validate:"required,max=50"`
	Rate int    `json:"rate" validate:"required"`
}

type UpdateProretyDTO struct {
	ID int       `json:"id" validate:"required"`
	Icon string  `json:"icon" validate:"omitempty"`
	Name string  `json:"name" validate:"omitempty"`
	Rate int     `json:"rate" validate:"omitempty"`
}


type ProretyDTO struct {
	ID int            `json:"id"`
	Icon string       `json:"icon"`
	Name string       `json:"name"`
	Rate int          `json:"rate"`
	CreatedAt string  `json:"created_at"` 
   
	UpdatedAt string  `json:"updated_at"` 
}


type ShortProretyDTO struct {
	ID int       `json:"id"`
	Name string  `json:"name"`
	
}