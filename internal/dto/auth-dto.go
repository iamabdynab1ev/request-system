package dto

type LoginDTO struct {
	Login    string `json:"login" validate:"required"`
	Password string `json:"password" validate:"required,min=6"`
}

type SendCodeDTO struct {
	Email string `json:"email" validate:"omitempty,email"`
	Phone string `json:"phone" validate:"omitempty,e164_TJ"`
}

type VerifyCodeDTO struct {
	Email string `json:"email" validate:"omitempty,email"`
	Phone string `json:"phone" validate:"omitempty,e164_TJ"`
	Code  string `json:"code" validate:"required,len=4,numeric"`
}

type ForgotPasswordDTO struct {
	Email string `json:"email" validate:"required,email"`
}

type ResetPasswordDTO struct {
	Token       string `json:"token" validate:"required"`
	NewPassword string `json:"new_password" validate:"required,min=6"`
}

type AuthResponseDTO struct {
	AccessToken  string        `json:"accessToken"`
	RefreshToken string        `json:"refreshToken"`
	User         UserPublicDTO `json:"user"`
}

type UserPublicDTO struct {
	ID     int    `json:"id"`
	Email  string `json:"email"`
	Phone  string `json:"phone,omitempty"`
	Fio    string `json:"fio"`
	RoleID int    `json:"role_id"`
}
