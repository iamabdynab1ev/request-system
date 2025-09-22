package entities

import (
	"time"

	"request-system/pkg/types"
)

type Branch struct {
	ID          uint64
	Name        string
	ShortName   string
	Address     string
	PhoneNumber string
	Email       string
	EmailIndex  string
	OpenDate    time.Time
	StatusID    uint64
	Status      *Status

	types.BaseEntity
}
