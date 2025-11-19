package dto

type ShortBranchDTO struct {
	ID        uint64 `json:"id"`
	Name      string `json:"name"`
	ShortName string `json:"short_name"`
}

type ShortOfficeDTO struct {
	ID   uint64 `json:"id"`
	Name string `json:"name"`
}
type ShortStatusDTO struct {
	ID   uint64 `json:"id"`
	Name string `json:"name"`
}
