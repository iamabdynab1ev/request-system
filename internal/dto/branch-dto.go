package dto


type CreateBranchDTO struct {
	Name        string `json:"name" validate:"required,max=50"`
	ShortName   string `json:"short_name" validate:"required,max=50"`
	Address     string `json:"address" validate:"required,max=150"`
	PhoneNumber string `json:"phone_number" validate:"required,len=12,regexp=^[0-9]{12}$"`
	Email       string `json:"email" validate:"email,omitempty"`
	OpenDate    string `json:"open_date" validate:"required"`
	StatusID    int    `json:"status_id" validate:"required"`
}

type UpdateBranchDTO struct {
	ID          int    `json:"id" validate:"required"`
	Name        string `json:"name" validate:"omitempty,max=50"`
	ShortName   string `json:"short_name" validate:"omitempty,max=50"`
	Address     string `json:"address" validate:"omitempty,max=150"`
	PhoneNumber string `json:"phone_number" validate:"omitempty,len=12,regexp=^[0-9]{12}$"`
	Email       string `json:"email" validate:"omitempty,email"`
	OpenDate    string `json:"open_date" validate:"omitempty"`
	StatusID    int    `json:"status_id" validate:"omitempty"`
}

type BranchDTO struct {
	ID          int              `json:"id"`
	Name        string           `json:"name"`
	ShortName   string           `json:"short_name"`
	Address     string           `json:"address"`
	PhoneNumber string           `json:"phone_number"`
	Email       string           `json:"email"`
	OpenDate    string           `json:"open_date"`
	Status      ShortStatusDTO   `json:"status"`
	CreatedAt   string           `json:"created_at"`
}


type ShortBranchDTO struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	ShortName string `json:"short_name"`
}
