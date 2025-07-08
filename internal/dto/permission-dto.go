// Package dto содержит структуры передачи данных.
package dto

type PermissionDTO struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type CreatePermissionDTO struct {
	Name        string `json:"name" validate:"required,max=100"`
	Description string `json:"description" validate:"required"`
}

type UpdatePermissionDTO struct {
	Name        string `json:"name" validate:"required,max=100"`
	Description string `json:"description" validate:"required"`
}

type ShortPermissionDTO struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}
