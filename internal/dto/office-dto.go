package dto

type CreateOfficeDTO struct {
	Name      string `json:"name" validate:"required, max=50"`
	Address   string `json:"address" validate:"required, max=150"`
	BranchID  int    `json:"branch_id" validate:"required"`
	StatusID  int   `json:"status_id" validate:"required"`
}

type UpdateOfficeDTO struct {

	ID        int    `json:"id" validate:"required"`
	Name      string `json:"name" validate:"omitempty, max=50"`
	Address   string `json:"address" validate:"omitempty, max=150"`
	OpenDate  string `json:"open_date" validate:"omitempty"`
	BranchID  int    `json:"branch_id" validate:"omitempty"`
	StatusID  int    `json:"status_id" validate:"omitempty"`
}

type OfficeDTO struct {
	ID        int            `json:"id"`
	Name      string         `json:"name"`
	Address   string         `json:"address"`
	OpenDate  string         `json:"open_date"`
	Branch    ShortBranchDTO `json:"branch"`
	Status    ShortStatusDTO `json:"status"`
	CreatedAt string         `json:"created_at"`
}

type ShortOfficeDTO struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}



