package entities

import "request-system/pkg/types"

type Status struct {
	ID        uint64  `json:"id"`
	IconSmall *string `json:"icon_small"`
	Name      string  `json:"name"`
	Type      int     `json:"type"`
	Code      *string `json:"code"`
	IconBig   *string `json:"icon_big"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`

	types.BaseEntity
}
