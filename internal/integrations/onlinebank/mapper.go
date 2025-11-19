package onlinebank

import (
	"fmt"
	"time"

	// Даем псевдоним `internalDTO`, чтобы не путаться с локальными DTO
	internalDTO "request-system/internal/integrations/dto"
	"request-system/pkg/utils"
)

// mapBranchToInternal — переводит одну внешнюю структуру филиала во внутреннюю.
func mapBranchToInternal(extBranch BranchDTO) (internalDTO.IntegrationBranchDTO, error) {
	openDate, err := time.Parse("2006-01-02T15:04:05", extBranch.OpenDate)
	if err != nil {
		return internalDTO.IntegrationBranchDTO{}, fmt.Errorf("неверный формат даты для филиала ID %d: %w", extBranch.ID, err)
	}
	normalizedPhone := utils.NormalizeTajikPhoneNumber(extBranch.Phone)
	return internalDTO.IntegrationBranchDTO{
		ExternalID:  fmt.Sprintf("onlinebank-%d", extBranch.ID),
		Name:        extBranch.Name,
		ShortName:   extBranch.ShortName,
		Address:     extBranch.Address,
		PhoneNumber: normalizedPhone,
		OpenDate:    openDate,
		IsActive:    extBranch.CloseDate == nil, // Наше бизнес-правило
	}, nil
}

// mapOfficeToInternal — переводит одну внешнюю структуру офиса во внутреннюю.
func mapOfficeToInternal(extOffice OfficeDTO) (internalDTO.IntegrationOfficeDTO, error) {
	// 1. Парсим дату открытия. Если формат неверный, возвращаем ошибку.
	openDate, err := time.Parse("2006-01-02T15:04:05", extOffice.OpenDate)
	if err != nil {
		return internalDTO.IntegrationOfficeDTO{}, fmt.Errorf("неверный формат даты для офиса ID %d: %w", extOffice.ID, err)
	}

	return internalDTO.IntegrationOfficeDTO{
		ExternalID:       fmt.Sprintf("onlinebank-%d", extOffice.ID),
		Name:             extOffice.Name,
		Address:          extOffice.Address,
		OpenDate:         openDate,
		BranchExternalID: fmt.Sprintf("onlinebank-%d", extOffice.BranchID),
		IsActive:         extOffice.CloseDate == nil,
	}, nil
}
