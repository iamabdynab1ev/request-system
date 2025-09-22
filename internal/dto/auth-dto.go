// Файл: internal/dto/auth.go
package dto

type LoginDTO struct {
	Login      string `json:"login" validate:"required"`
	Password   string `json:"password" validate:"required,min=6"`
	RememberMe bool   `json:"rememberMe"` // <<< ДОБАВЛЕНО: Принимаем флаг "Запомнить меня"
}

type ResetPasswordRequestDTO struct {
	Login string `json:"login" validate:"required"`
}

type VerifyCodeDTO struct {
	Login string `json:"login" validate:"required"`
	Code  string `json:"code"  validate:"required,len=4,numeric"`
}

type VerifyCodeResponseDTO struct {
	VerificationToken string `json:"verification_token"`
}

type ResetPasswordDTO struct {
	Token       string `json:"token"        validate:"required"`
	NewPassword string `json:"new_password" validate:"required,min=6"`
}

type AuthResponseDTO struct {
	AccessToken string   `json:"accessToken"`
	RoleName    string   `json:"role_name"`
	Permissions []string `json:"permissions"`
}

type UserProfileDTO struct {
	ID           uint64  `json:"id"`
	Email        string  `json:"email"`
	Phone        string  `json:"phone_number,omitempty"`
	FIO          string  `json:"fio"`
	RoleName     string  `json:"-"`
	PhotoURL     *string `json:"photo_url,omitempty"`
	Position     string  `json:"position"`
	BranchID     uint64  `json:"branch_id"`
	DepartmentID uint64  `json:"department_id"`
	OfficeID     *uint64 `json:"office_id,omitempty"`
	OtdelID      *uint64 `json:"otdel_id,omitempty"`
}

type ChangePasswordRequiredDTO struct {
	ResetToken string `json:"reset_token"`
	Message    string `json:"message"`
}
