package config

type UploadConfig struct {
	AllowedMimeTypes []string
	MaxSizeMB        int64
	MinWidth         int
	MaxWidth         int
	MinHeight        int
	MaxHeight        int
	PathPrefix       string
}

var UploadContexts = map[string]UploadConfig{
	"profile_photo": {
		AllowedMimeTypes: []string{"image/jpeg", "image/png", "image/gif", "image/webp", "image/jpg"},
		MaxSizeMB:        20,
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
	"icon_small": {
		AllowedMimeTypes: []string{"image/png", "image/svg+xml", "image/jpeg", "image/gif", "image/jpg"},
		MaxSizeMB:        5,

		PathPrefix: "icons/small",
	},
	"icon_big": {
		AllowedMimeTypes: []string{"image/png", "image/svg+xml", "image/jpeg", "image/gif", "image/jpg"},
		MaxSizeMB:        5,

		PathPrefix: "icons/big",
	},
}
