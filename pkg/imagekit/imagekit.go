package imagekit

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/imagekit-developer/imagekit-go/v2"
	"github.com/imagekit-developer/imagekit-go/v2/option"
	"github.com/raflytch/careerly-server/pkg/validator"
)

type Config struct {
	PublicKey   string
	PrivateKey  string
	URLEndpoint string
}

type Client struct {
	ik        imagekit.Client
	validator *validator.FileValidator
}

type UploadResult struct {
	URL       string `json:"url"`
	FileID    string `json:"file_id"`
	Name      string `json:"name"`
	Size      int64  `json:"size"`
	FileType  string `json:"file_type"`
	Thumbnail string `json:"thumbnail"`
}

func NewClient(config Config) *Client {
	ik := imagekit.NewClient(
		option.WithPrivateKey(config.PrivateKey),
	)

	return &Client{
		ik:        ik,
		validator: validator.ImageValidator(),
	}
}

func (c *Client) SetValidator(v *validator.FileValidator) {
	c.validator = v
}

func (c *Client) ValidateImage(file *multipart.FileHeader) error {
	return c.validator.Validate(file)
}

func (c *Client) UploadFile(ctx context.Context, file *multipart.FileHeader, folder string) (*UploadResult, error) {
	if err := c.ValidateImage(file); err != nil {
		return nil, err
	}

	src, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer src.Close()

	ext := strings.ToLower(filepath.Ext(file.Filename))
	uniqueFileName := fmt.Sprintf("%s_%d%s", uuid.New().String(), time.Now().Unix(), ext)

	resp, err := c.ik.Files.Upload(ctx, imagekit.FileUploadParams{
		File:     io.Reader(src),
		FileName: uniqueFileName,
		Folder:   imagekit.String(folder),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to upload file to ImageKit: %w", err)
	}

	return &UploadResult{
		URL:      resp.URL,
		FileID:   resp.FileID,
		Name:     resp.Name,
		Size:     int64(resp.Size),
		FileType: resp.FileType,
	}, nil
}

func (c *Client) DeleteFile(ctx context.Context, fileID string) error {
	if fileID == "" {
		return nil
	}

	err := c.ik.Files.Delete(ctx, fileID)
	if err != nil {
		return fmt.Errorf("failed to delete file from ImageKit: %w", err)
	}

	return nil
}
