// config/upload_config.go

package config

type UploadConfig struct {
	AllowedMimeTypes []string
	MaxSizeMB        int64
	ExpectedWidth    int
	ExpectedHeight   int

	PathPrefix string
}

var UploadContexts = map[string]UploadConfig{
	"profile_photo": {
		AllowedMimeTypes: []string{"image/jpeg", "image/png", "image/gif", "image/webp"},
		MaxSizeMB:        5,
		PathPrefix:       "avatars", // <---
	},
	"order_document": {
		AllowedMimeTypes: []string{
			"image/jpeg", "image/png", "application/pdf", // ... и другие ...
		},
		MaxSizeMB:  20,
		PathPrefix: "orders", // <---
	},
	"status_icon_small": {
		AllowedMimeTypes: []string{"image/png", "image/jpeg", "image/gif", "image/svg"},
		MaxSizeMB:        1,
		ExpectedWidth:    16,
		ExpectedHeight:   16,
		PathPrefix:       "icons", // <---
	},
	"status_icon_big": {
		AllowedMimeTypes: []string{"image/png", "image/jpeg", "image/gif", "image/svg"},
		MaxSizeMB:        1,
		ExpectedWidth:    24,
		ExpectedHeight:   24,
		PathPrefix:       "icons", // <---
	},
}
