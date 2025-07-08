package entities

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

type Order struct {
	ID          int
	Name        string
	Description pgtype.Text
	Address     string
	Duration    pgtype.Text

	DepartmentID int
	ProretyID    int
	StatusID     int
	BranchID     int
	EquipmentID  int
	CreatorID    int

	OtdelID    pgtype.Int4
	OfficeID   pgtype.Int4
	ExecutorID pgtype.Int4

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt pgtype.Timestamp
}
