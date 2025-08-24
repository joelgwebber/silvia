package llm

import (
	"context"
	"fmt"

	"github.com/revrost/go-openrouter"
)

type Client struct {
	openRouterClient *openrouter.Client
}

func NewClient(apiKey string) *Client {
	client := openrouter.NewClient(apiKey)
	return &Client{
		openRouterClient: client,
	}
}

func (c *Client) Complete(ctx context.Context, prompt string, model string) (string, error) {
	if model == "" {
		model = "openai/gpt-3.5-turbo"
	}

	request := openrouter.ChatCompletionRequest{
		Model: model,
		Messages: []openrouter.ChatCompletionMessage{
			{
				Role:    openrouter.ChatMessageRoleUser,
				Content: openrouter.Content{Text: prompt},
			},
		},
	}

	response, err := c.openRouterClient.CreateChatCompletion(ctx, request)
	if err != nil {
		return "", fmt.Errorf("failed to create completion: %w", err)
	}

	if len(response.Choices) == 0 {
		return "", fmt.Errorf("no completion choices returned")
	}

	return response.Choices[0].Message.Content.Text, nil
}

func (c *Client) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string, model string) (string, error) {
	if model == "" {
		model = "openai/gpt-3.5-turbo"
	}

	request := openrouter.ChatCompletionRequest{
		Model: model,
		Messages: []openrouter.ChatCompletionMessage{
			{
				Role:    openrouter.ChatMessageRoleSystem,
				Content: openrouter.Content{Text: systemPrompt},
			},
			{
				Role:    openrouter.ChatMessageRoleUser,
				Content: openrouter.Content{Text: userPrompt},
			},
		},
	}

	response, err := c.openRouterClient.CreateChatCompletion(ctx, request)
	if err != nil {
		return "", fmt.Errorf("failed to create completion: %w", err)
	}

	if len(response.Choices) == 0 {
		return "", fmt.Errorf("no completion choices returned")
	}

	return response.Choices[0].Message.Content.Text, nil
}

func (c *Client) ListModels(ctx context.Context) ([]string, error) {
	models, err := c.openRouterClient.ListModels(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list models: %w", err)
	}

	var modelNames []string
	for _, model := range models {
		modelNames = append(modelNames, model.ID)
	}

	return modelNames, nil
}