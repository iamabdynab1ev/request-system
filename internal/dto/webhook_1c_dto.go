// Файл: internal/dto/webhook_1c_dto.go
package dto

import "time"

// Webhook1CPayloadDTO это корневая структура для JSON-запроса от 1С.
type Webhook1CPayloadDTO struct {
	Departments []Department1CDTO `json:"departments"`
	Otdels      []Otdel1CDTO      `json:"otdels"`
	Branches    []Branch1CDTO     `json:"branches"`
	Offices     []Office1CDTO     `json:"offices"`
	Positions   []Position1CDTO   `json:"positions"`
	Users       []User1CDTO       `json:"users"`
}

// Department1CDTO представляет один департамент из 1С.
type Department1CDTO struct {
	ExternalID string `json:"externalId"`
	Name       string `json:"name"`
	IsActive   bool   `json:"isActive"`
}

// Otdel1CDTO представляет один отдел из 1С.
type Otdel1CDTO struct {
	ExternalID           string `json:"externalId"`
	Name                 string `json:"name"`
	DepartmentExternalID string `json:"departmentExternalId"`
	IsActive             bool   `json:"isActive"`
}

// Branch1CDTO представляет один филиал из 1С.
type Branch1CDTO struct {
	ExternalID  string    `json:"externalId"`
	Name        string    `json:"name"`
	ShortName   string    `json:"shortName"`
	Address     string    `json:"address"`
	PhoneNumber string    `json:"phoneNumber"`
	Email       string    `json:"email"`
	EmailIndex  string    `json:"emailIndex"`
	OpenDate    time.Time `json:"openDate"`
	IsActive    bool      `json:"isActive"`
}

// Office1CDTO представляет один офис из 1С.
type Office1CDTO struct {
	ExternalID       string    `json:"externalId"`
	Name             string    `json:"name"`
	Address          string    `json:"address"`
	OpenDate         time.Time `json:"openDate"`
	BranchExternalID string    `json:"branchExternalId"`
	IsActive         bool      `json:"isActive"`
}

// Position1CDTO представляет одну должность из 1С.
type Position1CDTO struct {
	ExternalID           string  `json:"externalId"`
	Name                 string  `json:"name"`
	PositionType         *string `json:"positionType"`
	IsActive             bool    `json:"isActive"`
	DepartmentExternalID *string `json:"departmentExternalId,omitempty"`
	OtdelExternalID      *string `json:"otdelExternalId,omitempty"`
	BranchExternalID     *string `json:"branchExternalId,omitempty"`
	OfficeExternalID     *string `json:"officeExternalId,omitempty"`
}

// User1CDTO представляет одного пользователя из 1С.
type User1CDTO struct {
	ExternalID           string  `json:"externalId"`
	Fio                  string  `json:"fio"`
	Email                string  `json:"email"`
	PhoneNumber          string  `json:"phoneNumber"`
	IsActive             bool    `json:"isActive"`
	PositionExternalID   string  `json:"positionExternalId"`
	DepartmentExternalID *string `json:"departmentExternalId,omitempty"`
	OtdelExternalID      *string `json:"otdelExternalId,omitempty"`
	BranchExternalID     *string `json:"branchExternalId,omitempty"`
	OfficeExternalID     *string `json:"officeExternalId,omitempty"`
}
