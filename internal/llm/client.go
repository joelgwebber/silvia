package llm

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/revrost/go-openrouter"
	"github.com/revrost/go-openrouter/jsonschema"
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

// CompleteWithStructuredOutput completes with a JSON schema for structured output
// The result parameter should be a pointer to a struct that will be populated with the response.
// Use this when you have a well-defined output structure and want schema validation.
func (c *Client) CompleteWithStructuredOutput(ctx context.Context, systemPrompt, userPrompt string, result interface{}, model string) error {
	if model == "" {
		model = "openai/gpt-4-turbo"  // Use a model that supports JSON mode well
	}

	// Generate JSON schema from the output type
	schema, err := jsonschema.GenerateSchemaForType(result)
	if err != nil {
		return fmt.Errorf("failed to generate schema: %w", err)
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
		ResponseFormat: &openrouter.ChatCompletionResponseFormat{
			Type: openrouter.ChatCompletionResponseFormatTypeJSONSchema,
			JSONSchema: &openrouter.ChatCompletionResponseFormatJSONSchema{
				Name:   "result",
				Schema: schema,
				Strict: false,  // Some models don't support strict mode
			},
		},
	}

	response, err := c.openRouterClient.CreateChatCompletion(ctx, request)
	if err != nil {
		return fmt.Errorf("failed to create structured completion: %w", err)
	}

	if len(response.Choices) == 0 {
		return fmt.Errorf("no completion choices returned")
	}

	// Unmarshal directly into the result
	if err := json.Unmarshal([]byte(response.Choices[0].Message.Content.Text), result); err != nil {
		return fmt.Errorf("failed to unmarshal structured response: %w", err)
	}

	return nil
}

// CompleteWithJSONMode completes with JSON mode enabled (less strict than schema).
// Use this when you need JSON output but don't have a predefined schema or need more flexibility.
// Returns the raw JSON string for manual parsing.
func (c *Client) CompleteWithJSONMode(ctx context.Context, systemPrompt, userPrompt string, model string) (string, error) {
	if model == "" {
		model = "openai/gpt-4-turbo"
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
		ResponseFormat: &openrouter.ChatCompletionResponseFormat{
			Type: openrouter.ChatCompletionResponseFormatTypeJSONObject,
		},
	}

	response, err := c.openRouterClient.CreateChatCompletion(ctx, request)
	if err != nil {
		return "", fmt.Errorf("failed to create JSON completion: %w", err)
	}

	if len(response.Choices) == 0 {
		return "", fmt.Errorf("no completion choices returned")
	}

	// Validate that response is valid JSON
	var test json.RawMessage
	if err := json.Unmarshal([]byte(response.Choices[0].Message.Content.Text), &test); err != nil {
		return "", fmt.Errorf("response is not valid JSON: %w", err)
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

func (c *Client) MergeEntities(ctx context.Context, entity1Content, entity2Content string, model string) (string, error) {
	if model == "" {
		model = "anthropic/claude-3.5-sonnet"
	}

	systemPrompt := `You are a knowledge graph entity merger. Your task is to merge two entity descriptions into a single, coherent entity that preserves ALL information, references, and relationships from both inputs.

CRITICAL REQUIREMENTS:
1. Preserve ALL wiki-links in [[entity-id]] format from both entities
2. Preserve ALL factual information from both entities
3. Preserve ALL sources listed from both entities
4. Preserve ALL relationships and connections mentioned
5. Remove only truly redundant information (exact duplicates)
6. Organize the merged content coherently with appropriate sections
7. Do NOT add any new information not present in either source
8. Do NOT remove any unique information from either source
9. Maintain a neutral, encyclopedic tone

Output the merged content in markdown format without frontmatter (that will be handled separately).`

	userPrompt := fmt.Sprintf(`Please merge these two entity descriptions:

ENTITY 1:
%s

ENTITY 2:
%s

Provide the merged content:`, entity1Content, entity2Content)

	return c.CompleteWithSystem(ctx, systemPrompt, userPrompt, model)
}
