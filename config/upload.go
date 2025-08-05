package config

// UploadConfig описывает правила для одного типа загрузки.
type UploadConfig struct {
	AllowedMimeTypes []string // Список разрешенных MIME-типов
	MaxSizeMB        int64    // Максимальный размер файла в мегабайтах
}

// UploadContexts - это наша главная карта, "мозг" всей системы.
// Ключ - это "контекст" загрузки, который фронтенд будет передавать в URL.
// Значение - правила для этого контекста.
var UploadContexts = map[string]UploadConfig{
	"profile_photo": {
		AllowedMimeTypes: []string{"image/jpeg", "image/png", "image/gif", "image/webp"},
		MaxSizeMB:        5, // Максимум 5 МБ для аватарок
	},
	"order_document": {
		AllowedMimeTypes: []string{
			"image/jpeg", "image/png",
			"application/pdf",
			"application/msword", // .doc
			"application/vnd.openxmlformats-officedocument.wordprocessingml.document", // .docx
			"application/vnd.ms-excel", // .xls
			"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",      // .xlsx
		},
		MaxSizeMB: 20, // Максимум 20 МБ для документов
	},
 
}