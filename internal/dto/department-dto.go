package dto



type CreateDepartmentDTO struct {
	Name string `json:"name" validate:"required,max=50"`

}


type UpdateDepartmentDTO struct {
    ID     int    `json:"id" validate:"required"`
    Name   string `json:"name" validate:"required,max=50"`
}


type DepartmentDTO struct {
	ID        int            `json:"id"`
	Name      string         `json:"name"`
	Status    ShortStatusDTO `json:"status"`
	CreatedAt string         `json:"created_at"`
}


type ShortDepartmentDTO struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}