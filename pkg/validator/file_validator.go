package validator

import (
	"fmt"
	"mime/multipart"
	"path/filepath"
	"strings"
)

const (
	KB int64 = 1024
	MB int64 = 1024 * KB
	GB int64 = 1024 * MB
)

const (
	MaxSize1MB  int64 = 1 * MB
	MaxSize2MB  int64 = 2 * MB
	MaxSize5MB  int64 = 5 * MB
	MaxSize10MB int64 = 10 * MB
)

type FileValidator struct {
	maxSize      int64
	allowedTypes map[string]bool
}

type FileValidatorOption func(*FileValidator)

func NewFileValidator(opts ...FileValidatorOption) *FileValidator {
	v := &FileValidator{
		maxSize:      MaxSize2MB,
		allowedTypes: make(map[string]bool),
	}

	for _, opt := range opts {
		opt(v)
	}

	return v
}

func WithMaxSize(size int64) FileValidatorOption {
	return func(v *FileValidator) {
		v.maxSize = size
	}
}

func WithAllowedTypes(types []string) FileValidatorOption {
	return func(v *FileValidator) {
		v.allowedTypes = make(map[string]bool)
		for _, t := range types {
			ext := strings.ToLower(t)
			if !strings.HasPrefix(ext, ".") {
				ext = "." + ext
			}
			v.allowedTypes[ext] = true
		}
	}
}

func WithImageTypes() FileValidatorOption {
	return func(v *FileValidator) {
		v.allowedTypes = map[string]bool{
			".jpg":  true,
			".jpeg": true,
			".png":  true,
			".gif":  true,
			".webp": true,
		}
	}
}

func WithDocumentTypes() FileValidatorOption {
	return func(v *FileValidator) {
		v.allowedTypes = map[string]bool{
			".pdf":  true,
			".doc":  true,
			".docx": true,
			".xls":  true,
			".xlsx": true,
			".ppt":  true,
			".pptx": true,
			".txt":  true,
		}
	}
}

func (v *FileValidator) Validate(file *multipart.FileHeader) error {
	if err := v.ValidateSize(file); err != nil {
		return err
	}

	if err := v.ValidateType(file); err != nil {
		return err
	}

	return nil
}

func (v *FileValidator) ValidateSize(file *multipart.FileHeader) error {
	if file.Size > v.maxSize {
		return fmt.Errorf("file size exceeds maximum limit of %s", v.formatSize(v.maxSize))
	}
	return nil
}

func (v *FileValidator) ValidateType(file *multipart.FileHeader) error {
	if len(v.allowedTypes) == 0 {
		return nil
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	if !v.allowedTypes[ext] {
		return fmt.Errorf("file type %s is not allowed. Allowed types: %s", ext, v.getAllowedTypesString())
	}
	return nil
}

func (v *FileValidator) GetMaxSize() int64 {
	return v.maxSize
}

func (v *FileValidator) GetAllowedTypes() []string {
	types := make([]string, 0, len(v.allowedTypes))
	for t := range v.allowedTypes {
		types = append(types, t)
	}
	return types
}

func (v *FileValidator) formatSize(size int64) string {
	if size >= GB {
		return fmt.Sprintf("%.2f GB", float64(size)/float64(GB))
	}
	if size >= MB {
		return fmt.Sprintf("%.0f MB", float64(size)/float64(MB))
	}
	if size >= KB {
		return fmt.Sprintf("%.0f KB", float64(size)/float64(KB))
	}
	return fmt.Sprintf("%d bytes", size)
}

func (v *FileValidator) getAllowedTypesString() string {
	types := make([]string, 0, len(v.allowedTypes))
	for t := range v.allowedTypes {
		types = append(types, strings.TrimPrefix(t, "."))
	}
	return strings.Join(types, ", ")
}

func ImageValidator() *FileValidator {
	return NewFileValidator(
		WithMaxSize(MaxSize2MB),
		WithImageTypes(),
	)
}

func ImageValidatorWithSize(maxSize int64) *FileValidator {
	return NewFileValidator(
		WithMaxSize(maxSize),
		WithImageTypes(),
	)
}

func DocumentValidator() *FileValidator {
	return NewFileValidator(
		WithMaxSize(MaxSize5MB),
		WithDocumentTypes(),
	)
}
