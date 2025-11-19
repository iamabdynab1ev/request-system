// Файл: internal/dto/auth.go
package dto

type LoginDTO struct {
	Login      string `json:"login" validate:"required"`
	Password   string `json:"password" validate:"required,min=6"`
	RememberMe bool   `json:"rememberMe"` 
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
	Permissions []string `json:"permissions"`
}

type UserProfileDTO struct {
	ID       uint64  `json:"id"`
	Email    string  `json:"email"`
	Phone    string  `json:"phone_number,omitempty"`
	FIO      string  `json:"fio"`
	PhotoURL *string `json:"photo_url,omitempty"`

	DepartmentID   uint64 `json:"department_id"`
	DepartmentName string `json:"department_name"`

	OtdelName *string `json:"otdel_name,omitempty"`

	PositionName string `json:"position_name"`

	BranchName string `json:"branch_name"`

	OfficeName *string `json:"office_name,omitempty"`
}
type ChangePasswordRequiredDTO struct {
	ResetToken string `json:"reset_token"`
	Message    string `json:"message"`
}

type UpdateMyProfileDTO struct {
	Fio         *string `json:"fio" validate:"omitempty,min=2"`
	PhoneNumber *string `json:"phone_number" validate:"omitempty"`
	Email       *string `json:"email" validate:"omitempty,email"`
	PhotoURL    *string `json:"photo_url,omitempty"`
}
