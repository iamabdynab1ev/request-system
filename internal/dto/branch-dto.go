package dto

type CreateBranchDTO struct {
	Name        string `json:"name" validate:"required"`
	ShortName   string `json:"short_name" validate:"required"`
	Address     string `json:"address" validate:"required"`
	PhoneNumber string `json:"phone_number" validate:"required"`
	Email       string `json:"email" validate:"required,email"`
	EmailIndex  string `json:"email_index" validate:"omitempty"`
	OpenDate    string `json:"open_date" validate:"required"`
	StatusID    uint64 `json:"status_id" validate:"required"`
}

type UpdateBranchDTO struct {
	Name        string `json:"name" validate:"omitempty"`
	ShortName   string `json:"short_name" validate:"omitempty"`
	Address     string `json:"address" validate:"omitempty"`
	PhoneNumber string `json:"phone_number" validate:"omitempty"`
	Email       string `json:"email" validate:"omitempty,email"`
	EmailIndex  string `json:"email_index" validate:"omitempty"`
	OpenDate    string `json:"open_date" validate:"omitempty"`
	StatusID    uint64 `json:"status_id" validate:"omitempty"`
}

type BranchDTO struct {
	ID          uint64         `json:"id"`
	Name        string         `json:"name"`
	ShortName   string         `json:"short_name"`
	Address     string         `json:"address"`
	PhoneNumber string         `json:"phone_number"`
	Email       string         `json:"email"`
	EmailIndex  string         `json:"email_index"`
	OpenDate    string         `json:"open_date"`
	Status      ShortStatusDTO `json:"status"`
	CreatedAt   string         `json:"created_at"`
	UpdatedAt   string         `json:"updated_at"`
}

type ShortBranchDTO struct {
	ID        uint64 `json:"id"`
	Name      string `json:"name"`
	ShortName string `json:"short_name"`
}
