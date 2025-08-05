package dto

type LoginDTO struct {
	Login    string `json:"login" validate:"required"`
	Password string `json:"password" validate:"required,min=6"`
}

type SendCodeDTO struct {
	Email string `json:"email" validate:"omitempty,email"`
	Phone string `json:"phone_number" validate:"omitempty,e164_TJ"`
}

type VerifyCodeDTO struct {
	Email string `json:"email" validate:"omitempty,email"`
	Phone string `json:"phone_number" validate:"omitempty,e164_TJ"`
	Code  string `json:"code" validate:"required,len=4,numeric"`
}

type ForgotPasswordInitDTO struct {
	Email string `json:"email" validate:"required,email"`
}

type ForgotPasswordOptionsDTO struct {
	Options []string `json:"options"`
}

type ForgotPasswordSendDTO struct {
	Email  string `json:"email"  validate:"required,email"`
	Method string `json:"method" validate:"required,oneof=email phone_number"`
}

type ResetPasswordEmailDTO struct {
	Token       string `json:"token"        validate:"required"`
	NewPassword string `json:"newPassword"  validate:"required,min=6"`
}

type ResetPasswordPhoneDTO struct {
	Email       string `json:"email"        validate:"required,email"`
	Code        string `json:"code"         validate:"required,len=4,numeric"`
	NewPassword string `json:"newPassword"  validate:"required,min=6"`
}

type AuthResponseDTO struct {
	AccessToken string        `json:"accessToken"`
	User        UserPublicDTO `json:"user"`
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
