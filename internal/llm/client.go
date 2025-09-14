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
		model = "openai/gpt-4-turbo" // Use a model that supports JSON mode well
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
				Strict: false, // Some models don't support strict mode
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

// FunctionCallRequest represents a request with function calling capabilities
type FunctionCallRequest struct {
	Model      string
	Messages   []Message
	Tools      []Tool
	ToolChoice string // "auto", "none", or specific tool name
}

// Message represents a chat message
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

// Tool represents a function that can be called
type Tool struct {
	Type     string   `json:"type"` // Always "function" for now
	Function Function `json:"function"`
}

// Function represents the function definition
type Function struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters"` // JSON Schema
}

// ToolCall represents a function call request from the model
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

// FunctionCall contains the function name and arguments
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}

// FunctionCallResponse represents the response from a function calling request
type FunctionCallResponse struct {
	Content   string
	ToolCalls []ToolCall
}

// CompleteWithFunctions performs a completion with function calling capabilities
func (c *Client) CompleteWithFunctions(ctx context.Context, request FunctionCallRequest) (*FunctionCallResponse, error) {
	if request.Model == "" {
		request.Model = "openai/gpt-4-turbo" // Default to a model that supports function calling
	}

	// Convert our messages to OpenRouter format
	orMessages := make([]openrouter.ChatCompletionMessage, len(request.Messages))
	for i, msg := range request.Messages {
		orMsg := openrouter.ChatCompletionMessage{
			Role:       msg.Role,
			Content:    openrouter.Content{Text: msg.Content},
			ToolCallID: msg.ToolCallID,
		}

		// Convert tool calls if present
		if len(msg.ToolCalls) > 0 {
			orMsg.ToolCalls = make([]openrouter.ToolCall, len(msg.ToolCalls))
			for j, tc := range msg.ToolCalls {
				orMsg.ToolCalls[j] = openrouter.ToolCall{
					ID:   tc.ID,
					Type: openrouter.ToolType(tc.Type),
					Function: openrouter.FunctionCall{
						Name:      tc.Function.Name,
						Arguments: tc.Function.Arguments,
					},
				}
			}
		}
		orMessages[i] = orMsg
	}

	// Convert our tools to OpenRouter format
	orTools := make([]openrouter.Tool, len(request.Tools))
	for i, tool := range request.Tools {
		orTools[i] = openrouter.Tool{
			Type: openrouter.ToolTypeFunction,
			Function: &openrouter.FunctionDefinition{
				Name:        tool.Function.Name,
				Description: tool.Function.Description,
				Parameters:  tool.Function.Parameters,
			},
		}
	}

	// Create the OpenRouter request
	orRequest := openrouter.ChatCompletionRequest{
		Model:      request.Model,
		Messages:   orMessages,
		Tools:      orTools,
		ToolChoice: request.ToolChoice,
	}

	// Make the API call
	response, err := c.openRouterClient.CreateChatCompletion(ctx, orRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to create completion with functions: %w", err)
	}

	if len(response.Choices) == 0 {
		return nil, fmt.Errorf("no completion choices returned")
	}

	choice := response.Choices[0]

	// Convert the response
	result := &FunctionCallResponse{
		Content: choice.Message.Content.Text,
	}

	// Convert tool calls if present
	if len(choice.Message.ToolCalls) > 0 {
		result.ToolCalls = make([]ToolCall, len(choice.Message.ToolCalls))
		for i, tc := range choice.Message.ToolCalls {
			result.ToolCalls[i] = ToolCall{
				ID:   tc.ID,
				Type: string(tc.Type),
				Function: FunctionCall{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				},
			}
		}
	}

	return result, nil
}
