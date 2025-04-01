package dto

type CreateRoleDTO struct {
	Name        string `json:"name" required:"max=50"`
	Description string `json:"description" validate:"required,max=100"`
	StatusID    int    `json:"status_id" validate:"required"`
}

type UpdateRoleDTO struct {
	ID          int    `json:"id" validate:"required"`
	Name        string `json:"name" validate:"omitempty,max=50"`
	Description string `json:"description" validate:"omitempty,max=100"`
	StatusID    int    `json:"status_id" validate:"omitemty"`
}

type RoleDTO struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      ShortStatusDTO `json:"status"`
	CreatedAt   string `json:"created_at"`
}

type ShortRoleDTO struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

