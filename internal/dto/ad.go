package dto

// ADUserDTO хранит информацию о пользователе, полученную из Active Directory.
type ADUserDTO struct {
	Username    string // sAMAccountName
	FullName    string // displayName
	Email       string // mail
	PhoneNumber string // telephoneNumber
	Department  string // department
	Position    string // title
}
