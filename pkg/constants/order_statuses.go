package constants

// --- СТАТУСЫ ЗАЯВОК (Совпадает с кодами в БД) ---
const (
	StatusActive        = "ACTIVE"
	StatusInactive      = "INACTIVE"
	StatusOpen          = "OPEN"
	StatusInProgress    = "IN_PROGRESS"
	StatusClosed        = "CLOSED"
	StatusRejected      = "REJECTED"
	StatusCompleted     = "COMPLETED"
	StatusRefinement    = "REFINEMENT"
	StatusClarification = "CLARIFICATION"
	StatusConfirmed     = "CONFIRMED"
	StatusService       = "SERVICE"
)

// Финальные статусы
var FinalStatuses = []string{
	StatusClosed,
	StatusCompleted,
	StatusRejected,
}

// Функция-проверка
func IsFinalStatus(code string) bool {
	for _, s := range FinalStatuses {
		if s == code {
			return true
		}
	}
	return false
}


const (
	OrderTypeEquipment      = "EQUIPMENT"
	OrderTypeAdministrative = "ADMINISTRATIVE"
)
