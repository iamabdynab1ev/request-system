package dto

import "time"

type DashboardFilterDTO struct {
	Period      string     `json:"period,omitempty"`
	DateFrom    *time.Time `json:"date_from,omitempty"`
	DateTo      *time.Time `json:"date_to,omitempty"`
	Widgets     []string   `json:"widgets,omitempty"`
	Granularity string     `json:"granularity,omitempty"`
}
