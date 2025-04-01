package services

import "request-system/internal/entities"

type BranchService struct{}

func NewBranchService() *BranchService {
	return &BranchService{}
}

func (service *BranchService) GetAll() ([]entities.Branch, error) {
	return []entities.Branch{}, nil
}

func (service *BranchService) GetByID(id string) (*entities.Branch, error) {
	return nil, nil
}
