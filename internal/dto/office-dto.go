package dto

type CreateOfficeDTO struct {
	Name       string `json:"name" validate:"required"`
	Address    string `json:"address" validate:"required"`
	OpenDate   string `json:"open_date" validate:"required"`
	BranchesID int    `json:"branches_id" validate:"required"`
	StatusID   int    `json:"status_id" validate:"required"`
}

type UpdateOfficeDTO struct {
	ID         int    `json:"id" validate:"required"`
	Name       string `json:"name" validate:"omitempty"`
	Address    string `json:"address" validate:"omitempty"`
	OpenDate   string `json:"open_date" validate:"omitempty"`
	BranchesID int    `json:"branches_id" validate:"omitempty"`
	StatusID   int    `json:"status_id" validate:"omitempty"`
}

type OfficeDTO struct {
	ID        int            `json:"id"`
	Name      string         `json:"name"`
	Address   string         `json:"address"`
	OpenDate  string         `json:"open_date"`
	Branch    ShortBranchDTO `json:"branch"`
	Status    ShortStatusDTO `json:"status"`
	CreatedAt string         `json:"created_at"`
	UpdatedAt string         `json:"updated_at"`
}

type ShortOfficeDTO struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}
