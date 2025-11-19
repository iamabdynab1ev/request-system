// Файл: internal/integrations/dto/branch.go
package dto

import "time"

type IntegrationBranchDTO struct {
	ExternalID  string
	Name        string
	ShortName   string
	Address     string
	PhoneNumber string
	OpenDate    time.Time
	Email       string
	IsActive    bool
}
