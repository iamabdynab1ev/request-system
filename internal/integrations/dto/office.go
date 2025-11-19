// Файл: internal/integrations/dto/office.go
package dto

import "time"

type IntegrationOfficeDTO struct {
	ExternalID       string
	Name             string
	Address          string
	OpenDate         time.Time
	BranchExternalID string
	IsActive         bool
}
