package mock

import (
	"context"

	"request-system/internal/integrations/dto"
)

type MockProvider struct {
	ShouldFail bool
}

func NewMockProvider() *MockProvider {
	return &MockProvider{}
}

func (m *MockProvider) Name() string {
	return "mock"
}

func (m *MockProvider) GetBranches(ctx context.Context) ([]dto.IntegrationBranchDTO, error) {
	if m.ShouldFail {
	}

	mockData := []dto.IntegrationBranchDTO{}
	return mockData, nil
}

func (m *MockProvider) GetOffices(ctx context.Context) ([]dto.IntegrationOfficeDTO, error) {
	if m.ShouldFail {
	}

	mockData := []dto.IntegrationOfficeDTO{}
	return mockData, nil
}
