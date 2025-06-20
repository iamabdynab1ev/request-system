package dto

import "request-system/internal/entities"

type LoginDto struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=6"`
}

type Token struct {
	AccessToken           string `json:"access_token"`
	RefreshToken          string `json:"refresh_token"`
	AccessTokenExpiredIn  int    `json:"access_token_expired_in"`
	RefreshTokenExpiredIn int    `json:"refresh_token_expired_in"`
}

type AuthResponse struct {
	Token Token         `json:"token"`
	User  entities.User `json:"user"`
}

type RegisterDto struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=6"`
}

type ChangePasswordDto struct {
	OldPassword string `json:"old_password" validate:"required"`
	NewPassword string `json:"new_password" validate:"required,min=6"`
}

type ForgetPasswordDto struct {
	Email string `json:"email" validate:"required,email"`
}

type ResetPasswordDto struct {
	Password string `json:"password" validate:"required,min=6"`
}

type CheckCodeDto struct {
	Code string `json:"code" validate:"required,len=6"`
}

type GenCodeDto struct {
	Email string `json:"email" validate:"required,email"`
}

type LogoutDto struct{}

type RefreshTokenDto struct{}
