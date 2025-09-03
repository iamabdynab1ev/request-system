package config

type UploadConfig struct {
	AllowedMimeTypes []string
	MaxSizeMB        int64
	MinWidth         int // Минимальная ширина
	MaxWidth         int // Максимальная ширина (0 - без лимита)
	MinHeight        int // Минимальная высота
	MaxHeight        int // Максимальная высота (0 - без лимита)
	PathPrefix       string
}

var UploadContexts = map[string]UploadConfig{
	"profile_photo": {
		AllowedMimeTypes: []string{"image/jpeg", "image/png", "image/gif", "image/webp", "image/jpg"},
		MaxSizeMB:        20,
		MinWidth:         50,
		MinHeight:        50,
		PathPrefix:       "avatars",
	},
	"order_document": {
		AllowedMimeTypes: []string{
			"image/jpeg", "image/png", "application/pdf", "image/jpg", "application/msword", "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
			"application/vnd.oasis.opendocument.text", "application/vnd.oasis.opendocument.presentation", "application/vnd.oasis.opendocument.spreadsheet",
		},
		MaxSizeMB:  20,
		PathPrefix: "orders",
	},
	// Универсальные правила для маленьких иконок
	"icon_small": {
		AllowedMimeTypes: []string{"image/png", "image/svg+xml", "image/jpeg", "image/gif", "image/jpg"},
		MaxSizeMB:        1,
		MinWidth:         16,
		MaxWidth:         32,
		MinHeight:        16,
		MaxHeight:        32,
		PathPrefix:       "icons/small",
	},
	// Универсальные правила для больших иконок
	"icon_big": {
		AllowedMimeTypes: []string{"image/png", "image/svg+xml", "image/jpeg", "image/gif", "image/jpg"},
		MaxSizeMB:        1,
		MinWidth:         24,
		MaxWidth:         64,
		MinHeight:        24,
		MaxHeight:        64,
		PathPrefix:       "icons/big",
	},
}
