package dto

type CreateCommentDTO struct {
	Message string `json:"message" validate:"required,min=1,max=1000"`
}

type UpdateCommentDTO struct {
	Message string `json:"message" validate:"omitempty,min=1,max=1000"`
}