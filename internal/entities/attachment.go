package entities

import "time"

type Attachment struct {
	ID        uint64    `db:"id"`         // ID вложения
	OrderID   uint64    `db:"order_id"`   // ID заявки
	UserID    uint64    `db:"user_id"`    // ID пользователя
	FileName  string    `db:"file_name"`  // Имя файла
	FilePath  string    `db:"file_path"`  // Путь к файлу
	FileType  string    `db:"file_type"`  // Тип файла
	FileSize  int64     `db:"file_size"`  // Размер файла
	CreatedAt time.Time `db:"created_at"` // Время создания
}
