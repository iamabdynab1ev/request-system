// Файл: internal/dto/auth.go
package dto

type LoginDTO struct {
	Login    string `json:"login" validate:"required"`
	Password string `json:"password" validate:"required,min=6"`
}

// Шаг 1: Запрос на сброс
type ResetPasswordRequestDTO struct {
	Login string `json:"login" validate:"required"`
}

// Шаг 2 (только телефон): Проверка кода
type VerifyCodeDTO struct { // <<< ПРАВИЛЬНОЕ ИМЯ
	Login string `json:"login" validate:"required"`
	Code  string `json:"code"  validate:"required,len=4,numeric"`
}

type VerifyCodeResponseDTO struct { // <<< ПРАВИЛЬНОЕ ИМЯ
	VerificationToken string `json:"verification_token"`
}

// Шаг 3: Установка нового пароля
type ResetPasswordDTO struct {
	Token       string `json:"token"        validate:"required"`
	NewPassword string `json:"new_password" validate:"required,min=6"`
}
type AuthResponseDTO struct {
	AccessToken string   `json:"accessToken"`
	Permissions []string `json:"permissions"`
}

type UserPublicDTO struct {
	ID           uint64  `json:"id"`
	Email        string  `json:"email"`
	Phone        string  `json:"phone_number,omitempty"`
	FIO          string  `json:"fio"`
	RoleID       uint64  `json:"role_id"`
	PhotoURL     *string `json:"photo_url,omitempty"`
	Position     string  `json:"position"`
	BranchID     uint64  `json:"branch_id"`
	DepartmentID uint64  `json:"department_id"`
	OfficeID     *uint64 `json:"officeId,omitempty"`
	OtdelID      *uint64 `json:"otdelId,omitempty"`
}

type UserProfileDTO struct {
	ID           uint64   `json:"id"`
	Email        string   `json:"email"`
	Phone        string   `json:"phone_number,omitempty"`
	FIO          string   `json:"fio"`
	RoleID       uint64   `json:"role_id"`
	Permissions  []string `json:"permissions"`
	PhotoURL     *string  `json:"photo_url,omitempty"`
	Position     string   `json:"position"`
	BranchID     uint64   `json:"branch_id"`
	DepartmentID uint64   `json:"department_id"`
	OfficeID     *uint64  `json:"officeId,omitempty"`
	OtdelID      *uint64  `json:"otdelId,omitempty"`
}
