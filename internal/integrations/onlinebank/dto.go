package onlinebank

// AuthResponse — структура для парсинга ответа с токеном.
type AuthResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

// BranchDTO — структура для парсинга одного филиала из их JSON-ответа.
type BranchDTO struct {
	ID        int     `json:"ID"`
	Name      string  `json:"Name"`
	ShortName string  `json:"ShortName"`
	Address   string  `json:"Address"`
	Phone     string  `json:"Phone"`
	OpenDate  string  `json:"OpenDate"`
	CloseDate *string `json:"CloseDate"`
}

// GetID добавляет соответствие интерфейсу для generic-функции.
func (b BranchDTO) GetID() int { return b.ID }

// OfficeDTO — структура для парсинга одного офиса (ЦБО) из их JSON-ответа.
type OfficeDTO struct {
	ID                      int     `json:"ID"`
	BranchID                int     `json:"BranchID"`
	Name                    string  `json:"Name"`
	ShortName               string  `json:"ShortName"`
	Address                 string  `json:"Address"`
	Phone                   string  `json:"Phone"`
	Fax                     string  `json:"Fax"`
	OpenDate                string  `json:"OpenDate"`
	CloseDate               *string `json:"CloseDate"`
	ManagerName             string  `json:"ManagerName"`
	ManagerPosition         string  `json:"ManagerPosition"`
	AttorneyNo              string  `json:"AttorneyNo"`
	AttorneyIssueDate       string  `json:"AttorneyIssueDate"`
	IsMainOffice            bool    `json:"IsMainOffice"`
	AdminDepartmentPosition string  `json:"AdminDepartmentPosition"`
	AdminDepartmentManager  string  `json:"AdminDepartmentManager"`
	CityID                  int     `json:"CityID"`
	Latitude                float64 `json:"Latitude"`
	Longitude               float64 `json:"Longitude"`
}


func (o OfficeDTO) GetID() int { return o.ID }
