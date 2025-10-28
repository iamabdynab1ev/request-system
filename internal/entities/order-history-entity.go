package entities

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

type OrderHistory struct {
	ID           uint64         `db:"id"`            // ID записи
	OrderID      uint64         `db:"order_id"`      // ID заявки
	OrderType    string         `db:"order_type"`    // Тип заявки
	UserID       uint64         `db:"user_id"`       // ID пользователя
	EventType    string         `db:"event_type"`    // Тип события
	OldValue     sql.NullString `db:"old_value"`     // Старое значение
	NewValue     sql.NullString `db:"new_value"`     // Новое значение
	Comment      sql.NullString `db:"comment"`       // Комментарий
	CreatedAt    time.Time      `db:"created_at"`    // Время создания
	AttachmentID sql.NullInt64  `db:"attachment_id"` // ID вложения
	Metadata     []byte         `db:"metadata"`      // Метаданные
	TxID         *uuid.UUID     `db:"tx_id"`         // ID транзакции
	CreatorFio   sql.NullString `db:"creator_fio"`   // ФИО создателя
	DelegatorFio sql.NullString `db:"delegator_fio"` // ФИО делегирующего
	ExecutorFio  sql.NullString `db:"executor_fio"`  // ФИО исполнителя
}
