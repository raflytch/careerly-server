package genai

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"

	"google.golang.org/genai"
)

type Client struct {
	client *genai.Client
	model  string
}

type Config struct {
	APIKey string
	Model  string
}

func NewClient(cfg Config) (*Client, error) {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: cfg.APIKey,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create genai client: %w", err)
	}

	model := cfg.Model
	if model == "" {
		model = "gemini-2.5-flash-lite"
	}

	return &Client{
		client: client,
		model:  model,
	}, nil
}

func (c *Client) GenerateText(ctx context.Context, prompt string) (string, error) {
	result, err := c.client.Models.GenerateContent(
		ctx,
		c.model,
		genai.Text(prompt),
		nil,
	)
	if err != nil {
		return "", fmt.Errorf("failed to generate content: %w", err)
	}
	return result.Text(), nil
}

func (c *Client) GenerateTextWithSystemPrompt(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	config := &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{
				{Text: systemPrompt},
			},
		},
	}

	result, err := c.client.Models.GenerateContent(
		ctx,
		c.model,
		genai.Text(userPrompt),
		config,
	)
	if err != nil {
		return "", fmt.Errorf("failed to generate content: %w", err)
	}
	return result.Text(), nil
}

func (c *Client) GenerateFromFile(ctx context.Context, file *multipart.FileHeader, prompt string) (string, error) {
	f, err := file.Open()
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	contents := []*genai.Content{
		{
			Parts: []*genai.Part{
				{Text: prompt},
				{
					InlineData: &genai.Blob{
						MIMEType: file.Header.Get("Content-Type"),
						Data:     data,
					},
				},
			},
		},
	}

	result, err := c.client.Models.GenerateContent(
		ctx,
		c.model,
		contents,
		nil,
	)
	if err != nil {
		return "", fmt.Errorf("failed to generate content from file: %w", err)
	}
	return result.Text(), nil
}

func (c *Client) GenerateFromFileWithSystemPrompt(ctx context.Context, file *multipart.FileHeader, systemPrompt, userPrompt string) (string, error) {
	f, err := file.Open()
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	contents := []*genai.Content{
		{
			Parts: []*genai.Part{
				{Text: userPrompt},
				{
					InlineData: &genai.Blob{
						MIMEType: file.Header.Get("Content-Type"),
						Data:     data,
					},
				},
			},
		},
	}

	config := &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{
				{Text: systemPrompt},
			},
		},
	}

	result, err := c.client.Models.GenerateContent(
		ctx,
		c.model,
		contents,
		config,
	)
	if err != nil {
		return "", fmt.Errorf("failed to generate content from file: %w", err)
	}
	return result.Text(), nil
}

func (c *Client) GenerateJSON(ctx context.Context, prompt string) (string, error) {
	config := &genai.GenerateContentConfig{
		ResponseMIMEType: "application/json",
	}

	result, err := c.client.Models.GenerateContent(
		ctx,
		c.model,
		genai.Text(prompt),
		config,
	)
	if err != nil {
		return "", fmt.Errorf("failed to generate json content: %w", err)
	}
	return result.Text(), nil
}

func (c *Client) GenerateJSONWithSystemPrompt(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	config := &genai.GenerateContentConfig{
		ResponseMIMEType: "application/json",
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{
				{Text: systemPrompt},
			},
		},
	}

	result, err := c.client.Models.GenerateContent(
		ctx,
		c.model,
		genai.Text(userPrompt),
		config,
	)
	if err != nil {
		return "", fmt.Errorf("failed to generate json content: %w", err)
	}
	return result.Text(), nil
}
