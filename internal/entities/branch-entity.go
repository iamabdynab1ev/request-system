package entities

import "request-system/pkg/types"
import "time"

type Branch struct {
	ID           int        `json:"id"`
	Name         string     `json:"name"`
	ShortName    string     `json:"short_name"`
	Address      string     `json:"address"`
	PhoneNumber  string     `json:"phone_number"`
	Email        string     `json:"email"`
	EmailIndex   string     `json:"email_index"`
	OpenDate     time.Time  `json:"open_date"`
	StatusID     int        `json:"status_id"`

	types.BaseEntity
}