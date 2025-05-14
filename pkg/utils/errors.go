package utils

import "errors"

var (
	ErrorNotFound   = errors.New("Данные не найдены")
	ErrorBadRequest = errors.New("Неверный запрос")
)

var ErrorList = map[error]int{
	ErrorNotFound:   404,
	ErrorBadRequest: 400,
}
