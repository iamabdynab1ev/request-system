package dto

type UserClaims struct {
	UserID      uint64
	RoleID      uint64
	Permissions []string
}
