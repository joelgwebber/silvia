package operations

import (
	"context"
	"encoding/json"
	"fmt"

	"silvia/internal/llm"
)

// LLMOps handles LLM-assisted operations including function calling
type LLMOps struct {
	llm *llm.Client
}

// NewLLMOps creates a new LLM operations handler
func NewLLMOps(llmClient *llm.Client) *LLMOps {
	return &LLMOps{
		llm: llmClient,
	}
}

// FunctionCallRequest represents a request for LLM function calling
type FunctionCallRequest struct {
	Messages   []LLMMessage
	Tools      []map[string]any
	ToolChoice string // "auto", "none", or specific tool name
}

// LLMMessage represents a message in the conversation
type LLMMessage struct {
	Role    string `json:"role"` // "user", "assistant", "system"
	Content string `json:"content"`
}

// FunctionCallResponse represents the response from LLM function calling
type FunctionCallResponse struct {
	Content    string
	ToolCalls  []ToolCall
	StopReason string
}

// ToolCall represents a single tool invocation request from the LLM
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

// FunctionCall contains the function name and arguments
type FunctionCall struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// ExecuteFunctionCall sends a request to the LLM with available tools and executes any requested functions
func (l *LLMOps) ExecuteFunctionCall(ctx context.Context, userInput string, toolSchemas []map[string]any) (*FunctionCallResponse, error) {
	if l.llm == nil {
		return nil, fmt.Errorf("LLM client not initialized")
	}

	// Convert tool schemas to LLM format
	tools := make([]llm.Tool, len(toolSchemas))
	for i, schema := range toolSchemas {
		tools[i] = llm.Tool{
			Type: "function",
			Function: llm.Function{
				Name:        schema["name"].(string),
				Description: schema["description"].(string),
				Parameters:  schema["parameters"],
			},
		}
	}

	// Create the LLM request
	llmRequest := llm.FunctionCallRequest{
		Model: "openai/gpt-4-turbo", // Use a model that supports function calling
		Messages: []llm.Message{
			{
				Role:    "user",
				Content: userInput,
			},
		},
		Tools:      tools,
		ToolChoice: "auto",
	}

	// Call the LLM with function calling
	llmResponse, err := l.llm.CompleteWithFunctions(ctx, llmRequest)
	if err != nil {
		return nil, fmt.Errorf("LLM function call failed: %w", err)
	}

	// Convert the response to our format
	response := &FunctionCallResponse{
		Content: llmResponse.Content,
	}

	if len(llmResponse.ToolCalls) > 0 {
		response.ToolCalls = make([]ToolCall, len(llmResponse.ToolCalls))
		for i, tc := range llmResponse.ToolCalls {
			response.ToolCalls[i] = ToolCall{
				ID:   tc.ID,
				Type: tc.Type,
				Function: FunctionCall{
					Name:      tc.Function.Name,
					Arguments: json.RawMessage(tc.Function.Arguments),
				},
			}
		}
	}

	return response, nil
}

// RefineEntityWithLLM uses the LLM to refine an entity's content
func (l *LLMOps) RefineEntityWithLLM(ctx context.Context, entityID string, currentContent string, instructions string) (string, error) {
	if l.llm == nil {
		return "", fmt.Errorf("LLM client not initialized")
	}

	prompt := fmt.Sprintf(`You are refining the content for entity: %s

Current content:
%s

Instructions: %s

Please provide the refined content, maintaining the same markdown format with wiki-links to other entities where appropriate.`, entityID, currentContent, instructions)

	response, err := l.llm.Complete(ctx, prompt, "")
	if err != nil {
		return "", fmt.Errorf("LLM refinement failed: %w", err)
	}

	return response, nil
}

// ExtractEntitiesFromText uses the LLM to extract entities from text
func (l *LLMOps) ExtractEntitiesFromText(ctx context.Context, text string, sourceURL string) ([]LLMExtractedEntity, error) {
	if l.llm == nil {
		return nil, fmt.Errorf("LLM client not initialized")
	}

	prompt := fmt.Sprintf(`Extract entities from the following text. Return as JSON array with objects containing:
- type: "person", "organization", "concept", "work", or "event"
- id: a kebab-case identifier
- name: the display name
- description: a brief description
- relationships: array of {target: "entity-id", type: "relationship-type"}

Source URL: %s

Text:
%s`, sourceURL, text)

	response, err := l.llm.Complete(ctx, prompt, "")
	if err != nil {
		return nil, fmt.Errorf("entity extraction failed: %w", err)
	}

	// Parse the JSON response
	var entities []LLMExtractedEntity
	if err := json.Unmarshal([]byte(response), &entities); err != nil {
		// If parsing fails, return empty list rather than error
		// The LLM might not always return valid JSON
		return []LLMExtractedEntity{}, nil
	}

	return entities, nil
}

// LLMExtractedEntity represents an entity extracted by the LLM
type LLMExtractedEntity struct {
	Type          string                     `json:"type"`
	ID            string                     `json:"id"`
	Name          string                     `json:"name"`
	Description   string                     `json:"description"`
	Relationships []LLMExtractedRelationship `json:"relationships"`
}

// LLMExtractedRelationship represents a relationship extracted by the LLM
type LLMExtractedRelationship struct {
	Target string `json:"target"`
	Type   string `json:"type"`
}
