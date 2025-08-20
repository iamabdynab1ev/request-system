package dto

// DTO для СОЗДАНИЯ филиала (принимаем от клиента)
type CreateBranchDTO struct {
	Name        string `json:"name" validate:"required"`
	ShortName   string `json:"short_name" validate:"required"`
	Address     string `json:"address" validate:"required"`
	PhoneNumber string `json:"phone_number" validate:"required"`
	Email       string `json:"email" validate:"required,email"`
	EmailIndex  string `json:"email_index" validate:"required"`
	OpenDate    string `json:"open_date" validate:"required"` // Дата в формате "YYYY-MM-DD"
	StatusID    uint64 `json:"status_id" validate:"required"`
}

// DTO для ОБНОВЛЕНИЯ филиала (принимаем от клиента)
type UpdateBranchDTO struct {
	Name        string `json:"name,omitempty"`
	ShortName   string `json:"short_name,omitempty"`
	Address     string `json:"address,omitempty"`
	PhoneNumber string `json:"phone_number,omitempty"`
	Email       string `json:"email,omitempty,email"`
	EmailIndex  string `json:"email_index,omitempty"`
	OpenDate    string `json:"open_date,omitempty"` // Дата в формате "YYYY-MM-DD"
	StatusID    uint64 `json:"status_id,omitempty"`
}

// >>> ИЗМЕНЕНИЕ <<<
// DTO для ответа в СПИСКЕ. Статус здесь - это просто ID.
type BranchListResponseDTO struct {
	ID          uint64 `json:"id"`
	Name        string `json:"name"`
	ShortName   string `json:"short_name"`
	Address     string `json:"address"`
	PhoneNumber string `json:"phone_number"`
	Email       string `json:"email"`
	EmailIndex  string `json:"email_index"`
	OpenDate    string `json:"open_date"`
	StatusID    uint64 `json:"status_id"` // <-- ДОБАВИЛИ ЭТО ПОЛЕ
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// >>> ИЗМЕНЕНИЕ <<<
// DTO для ДЕТАЛЬНОГО ответа (один филиал). Статус - это объект.
type BranchDTO struct {
	ID          uint64          `json:"id"`
	Name        string          `json:"name"`
	ShortName   string          `json:"short_name"`
	Address     string          `json:"address"`
	PhoneNumber string          `json:"phone_number"`
	Email       string          `json:"email"`
	EmailIndex  string          `json:"email_index"`
	OpenDate    string          `json:"open_date"`
	Status      *ShortStatusDTO `json:"status"` // <-- СДЕЛАЛИ ПОЛЕМ ОБЪЕКТ
	CreatedAt   string          `json:"created_at"`
	UpdatedAt   string          `json:"updated_at"`
}
type ShortBranchDTO struct {
	ID        uint64 `json:"id"`
	Name      string `json:"name"`
	ShortName string `json:"short_name"`
}
