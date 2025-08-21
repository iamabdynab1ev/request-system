// Файл: internal/services/office.go
// СКОПИРУЙТЕ И ПОЛНОСТЬЮ ЗАМЕНИТЕ СОДЕРЖИМОЕ

package services

import (
	"context"

	"request-system/internal/dto"
	"request-system/internal/repositories"
	"request-system/pkg/types" // <-- ДОБАВЛЕН ИМПОРТ

	"go.uber.org/zap"
)

const (
	timeFormat = "2006-01-02 15:04:05"
	dateFormat = "2006-01-02"
)

type OfficeService struct {
	officeRepository repositories.OfficeRepositoryInterface
	logger           *zap.Logger
}

func NewOfficeService(officeRepository repositories.OfficeRepositoryInterface, logger *zap.Logger) *OfficeService {
	return &OfficeService{
		officeRepository: officeRepository,
		logger:           logger,
	}
}

// "Переводчик" в DTO для ответа
func toOfficeResponseDTO(office *dto.OfficeDTO) *dto.OfficeResponseDTO {
	if office == nil {
		return nil
	}
	response := &dto.OfficeResponseDTO{
		ID:        office.ID,
		Name:      office.Name,
		Address:   office.Address,
		OpenDate:  office.OpenDate.Format(dateFormat),
		CreatedAt: office.CreatedAt.Format(timeFormat),
		UpdatedAt: office.UpdatedAt.Format(timeFormat),
	}
	if office.Branch != nil {
		response.BranchID = office.Branch.ID
	}
	if office.Status != nil {
		response.StatusID = office.Status.ID
	}
	return response
}

// ИЗМЕНЕНО: теперь принимает types.Filter
func (s *OfficeService) GetOffices(ctx context.Context, filter types.Filter) ([]dto.OfficeResponseDTO, uint64, error) {
	officesFromRepo, total, err := s.officeRepository.GetOffices(ctx, filter)
	if err != nil {
		s.logger.Error("Ошибка при получении офисов", zap.Error(err))
		return nil, 0, err
	}

	responseDTOs := make([]dto.OfficeResponseDTO, 0, len(officesFromRepo))
	for _, office := range officesFromRepo {
		responseDTOs = append(responseDTOs, *toOfficeResponseDTO(&office))
	}
	return responseDTOs, total, nil
}

func (s *OfficeService) FindOffice(ctx context.Context, id uint64) (*dto.OfficeResponseDTO, error) {
	office, err := s.officeRepository.FindOffice(ctx, id)
	if err != nil {
		return nil, err
	}
	return toOfficeResponseDTO(office), nil
}

func (s *OfficeService) CreateOffice(ctx context.Context, payload dto.CreateOfficeDTO) (*dto.OfficeResponseDTO, error) {
	newID, err := s.officeRepository.CreateOffice(ctx, payload)
	if err != nil {
		s.logger.Error("Ошибка при создании офиса", zap.Error(err))
		return nil, err
	}
	s.logger.Info("Офис успешно создан", zap.Uint64("id", newID))
	return s.FindOffice(ctx, newID)
}

func (s *OfficeService) UpdateOffice(ctx context.Context, id uint64, payload dto.UpdateOfficeDTO) (*dto.OfficeResponseDTO, error) {
	if _, err := s.officeRepository.FindOffice(ctx, id); err != nil {
		return nil, err
	}
	if err := s.officeRepository.UpdateOffice(ctx, id, payload); err != nil {
		s.logger.Error("Ошибка при обновлении офиса", zap.Error(err))
		return nil, err
	}
	s.logger.Info("Офис успешно обновлен", zap.Uint64("id", id))
	return s.FindOffice(ctx, id)
}

func (s *OfficeService) DeleteOffice(ctx context.Context, id uint64) error {
	err := s.officeRepository.DeleteOffice(ctx, id)
	if err != nil {
		s.logger.Error("Ошибка при удалении офиса", zap.Error(err))
		return err
	}
	s.logger.Info("Офис успешно удален", zap.Uint64("id", id))
	return nil
}
