// Файл: internal/dto/branch.go
// СКОПИРУЙТЕ И ПОЛНОСТЬЮ ЗАМЕНИТЕ СОДЕРЖИМОЕ

package dto

// CreateBranchDTO - структура для создания филиала
type CreateBranchDTO struct {
	Name        string `json:"name" validate:"required"`
	ShortName   string `json:"short_name" validate:"required"`
	Address     string `json:"address" validate:"required"`
	PhoneNumber string `json:"phone_number" validate:"required"`
	Email       string `json:"email" validate:"required,email"`
	EmailIndex  string `json:"email_index" validate:"omitempty"`
	OpenDate    string `json:"open_date" validate:"required,datetime=2006-01-02"`
	StatusID    uint64 `json:"status_id" validate:"required,gt=0"`
}

// UpdateBranchDTO - структура для обновления филиала
type UpdateBranchDTO struct {
	Name        string `json:"name" validate:"omitempty"`
	ShortName   string `json:"short_name" validate:"omitempty"`
	Address     string `json:"address" validate:"omitempty"`
	PhoneNumber string `json:"phone_number" validate:"omitempty"`
	Email       string `json:"email" validate:"omitempty,email"`
	EmailIndex  string `json:"email_index" validate:"omitempty"`
	OpenDate    string `json:"open_date" validate:"omitempty,datetime=2006-01-02"`
	StatusID    uint64 `json:"status_id" validate:"omitempty,gt=0"`
}

// BranchDTO - "ПОЛНАЯ" структура, которую возвращает репозиторий
type BranchDTO struct {
	ID          uint64
	Name        string
	ShortName   string
	Address     string
	PhoneNumber string
	Email       string
	EmailIndex  string
	OpenDate    string
	Status      *ShortStatusDTO // <--- ВАЖНО: поле называется Status, тип - объект
	CreatedAt   string
	UpdatedAt   string
}

// BranchListResponseDTO - "УПРОЩЕННАЯ" структура, которую мы отдаем в JSON
type BranchListResponseDTO struct {
	ID          uint64 `json:"id"`
	Name        string `json:"name"`
	ShortName   string `json:"short_name"`
	Address     string `json:"address"`
	PhoneNumber string `json:"phone_number"`
	Email       string `json:"email"`
	EmailIndex  string `json:"email_index"`
	OpenDate    string `json:"open_date"`
	Status      uint64 `json:"status_id"` // <--- ВОТ ВАШЕ ЖЕЛАНИЕ: поле status, тип - число
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type ShortBranchDTO struct {
	ID        uint64 `json:"id"`
	Name      string `json:"name"`
	ShortName string `json:"short_name"`
}
