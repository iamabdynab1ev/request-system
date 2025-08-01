// Package dto содержит структуры передачи данных.
package dto

import "time"

type PermissionDTO struct {
	ID          uint64    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
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
	ID   uint64 `json:"id"`
	Name string `json:"name"`
}
