package dto

import "time"

type CreateAttachmentDTO struct {
	OrderID  uint64 `json:"order_id" validate:"required"`
	FileName string `json:"file_name" validate:"required,max=255"`
	FilePath string `json:"file_path" validate:"required,max=500"`
	FileType string `json:"file_type" validate:"required,max=50"`
}

type UpdateAttachmentDTO struct {
	ID       uint64 `json:"id" validate:"required"`
	FileName string `json:"file_name" validate:"omitempty,max=255"`
	FilePath string `json:"file_path" validate:"omitempty,max=500"`
	FileType string `json:"file_type" validate:"omitempty,max=50"`
}

type AttachmentDTO struct {
	ID        uint64    `json:"id"`
	OrderID   uint64    `json:"order_id"`
	FileName  string    `json:"file_name"`
	FilePath  string    `json:"file_path"`
	FileType  string    `json:"file_type"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type AttachmentResponseDTO struct {
	ID       uint64 `json:"id"`
	FileName string `json:"file_name"`
	URL      string `json:"url"`
}

type AttachmentResponseListDTO struct {
	Attachments []AttachmentResponseDTO `json:"attachments"`
}
