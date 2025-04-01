package dto

type LoginDto struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=6"`
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
