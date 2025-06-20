package entities

import "request-system/pkg/types"

type Department struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	StatusID     int    `json:"status_id"`

	types.BaseEntity
}