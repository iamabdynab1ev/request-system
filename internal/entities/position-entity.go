// Файл: internal/entities/position-entity.go
package entities

import (
	"request-system/pkg/types"
)

type Position struct {
	Id       int     `json:"id"`
	Name     string  `json:"name"`
	Code     *string `json:"code"`
	Level    int     `json:"level"`
	StatusID int     `json:"status_id"`

	types.BaseEntity // Добавляем CreatedAt, UpdatedAt
}
