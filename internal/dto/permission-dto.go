package dto

type CreatePermissionDTO struct {
	Name        string `json:"name" validate:"required,max=50"`
	Description string `json:"description" validate:"required,max=100"`
}

type UpdatePermissionDTO struct {
	ID          int    `json:"id" validate:"required"`
	Name        string `json:"name" validate:"omitempty,max=50"`
	Description string `json:"description" validate:"omitempty,max=100"`
}

type PermissionDTO struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
}

type ShortPermissionDTO struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}
