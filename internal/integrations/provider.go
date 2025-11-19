package integrations

import (
	"context"

	"request-system/internal/integrations/dto"
)

type DataProvider interface {
	Name() string
	GetBranches(ctx context.Context) ([]dto.IntegrationBranchDTO, error)
	GetOffices(ctx context.Context) ([]dto.IntegrationOfficeDTO, error)
}
