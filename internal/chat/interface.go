// Package chat provides a natural language interface to the knowledge graph.
package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"silvia/internal/llm"
	"silvia/internal/tools"
)

// ConversationTurn represents a single turn in the conversation
type ConversationTurn struct {
	Role        string             // "user" or "assistant"
	Message     string             // The message content
	ToolCalls   []tools.ToolCall   // Tools called (for assistant turns)
	ToolResults []tools.ToolResult // Tool results (for assistant turns)
	Timestamp   time.Time          // When this turn occurred
}

// ChatInterface provides a natural language interface to the knowledge graph using tools.
// It ensures all responses are grounded in actual graph data, not general knowledge
type ChatInterface struct {
	tools      *tools.Manager
	llm        *llm.Client
	logger     *tools.DefaultLogger
	history    []ConversationTurn // Conversation history
	maxHistory int                // Maximum turns to keep in history
}

// NewChatInterface creates a new chat interface
func NewChatInterface(toolManager *tools.Manager, llmClient *llm.Client) *ChatInterface {
	// Create a logger for the chat interface
	logger := tools.NewDefaultLogger(true) // Verbose by default to track behavior

	// Enable logging on the tool manager
	toolManager.SetLogger(logger)

	return &ChatInterface{
		tools:      toolManager,
		llm:        llmClient,
		logger:     logger,
		history:    make([]ConversationTurn, 0),
		maxHistory: 10, // Keep last 10 turns (5 exchanges)
	}
}

// EnableLogging enables or disables verbose logging
func (c *ChatInterface) EnableLogging(verbose bool) {
	logger := tools.NewDefaultLogger(verbose)
	c.logger = logger
	c.tools.SetLogger(logger)
}

// ProcessMessage handles a natural language message from the user
func (c *ChatInterface) ProcessMessage(ctx context.Context, message string) (string, error) {
	c.logger.LogToolCall("CHAT", map[string]any{"message": message})

	// Add user message to history
	c.addToHistory("user", message, nil, nil)

	// Step 1: Use LLM to understand intent and extract tool calls (with context)
	toolCalls, err := c.extractToolCalls(ctx, message)
	if err != nil {
		c.logger.LogToolError("CHAT", fmt.Errorf("failed to extract tools: %w", err))
		return "", fmt.Errorf("failed to understand message: %w", err)
	}

	c.logger.LogToolCall("CHAT", map[string]any{"extracted_tools": toolCalls})

	// Step 2: Execute the tools
	var results []tools.ToolResult
	for _, call := range toolCalls {
		result, err := c.tools.Execute(ctx, call.Tool, call.Args)
		if err != nil {
			c.logger.LogToolError("CHAT", fmt.Errorf("tool %s failed: %w", call.Tool, err))
			return "", fmt.Errorf("tool execution failed: %w", err)
		}
		results = append(results, result)
	}

	// Step 3: Format the results into a natural language response (with context)
	response, err := c.formatResponse(ctx, message, toolCalls, results)
	if err != nil {
		c.logger.LogToolError("CHAT", fmt.Errorf("failed to format response: %w", err))
		return "", fmt.Errorf("failed to format response: %w", err)
	}

	// Add assistant response to history
	c.addToHistory("assistant", response, toolCalls, results)

	c.logger.LogToolCall("CHAT_RESPONSE", map[string]any{"response": response})

	return response, nil
}

// addToHistory adds a turn to the conversation history
func (c *ChatInterface) addToHistory(role, message string, toolCalls []tools.ToolCall, results []tools.ToolResult) {
	turn := ConversationTurn{
		Role:        role,
		Message:     message,
		ToolCalls:   toolCalls,
		ToolResults: results,
		Timestamp:   time.Now(),
	}

	c.history = append(c.history, turn)

	// Trim history if it exceeds max
	if len(c.history) > c.maxHistory {
		c.history = c.history[len(c.history)-c.maxHistory:]
	}
}

// ClearHistory clears the conversation history
func (c *ChatInterface) ClearHistory() {
	c.history = make([]ConversationTurn, 0)
}

// GetHistory returns the conversation history
func (c *ChatInterface) GetHistory() []ConversationTurn {
	return c.history
}

// max returns the maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// extractToolCalls uses the LLM to determine which tools to call
func (c *ChatInterface) extractToolCalls(ctx context.Context, message string) ([]tools.ToolCall, error) {
	// Build a prompt that includes available tools
	prompt := c.buildToolExtractionPrompt(message)

	// Get LLM response
	response, err := c.llm.Complete(ctx, prompt, "")
	if err != nil {
		return nil, err
	}

	// Parse the JSON response into tool calls
	var toolCalls []tools.ToolCall
	if err := json.Unmarshal([]byte(response), &toolCalls); err != nil {
		// If JSON parsing fails, try to extract tool calls from text
		toolCalls = c.parseToolCallsFromText(response)
	}

	return toolCalls, nil
}

// buildToolExtractionPrompt creates a prompt for the LLM to extract tool calls
func (c *ChatInterface) buildToolExtractionPrompt(message string) string {
	var prompt strings.Builder

	prompt.WriteString("You are a knowledge graph assistant that ONLY uses information from the local knowledge graph.\n")
	prompt.WriteString("You must use the provided tools to access entity data - do not use general knowledge from training.\n")

	// Include conversation history for context
	if len(c.history) > 0 {
		prompt.WriteString("\nRecent conversation history:\n")
		for i := max(0, len(c.history)-4); i < len(c.history); i++ { // Last 2 exchanges
			turn := c.history[i]
			prompt.WriteString(fmt.Sprintf("%s: %s\n", strings.Title(turn.Role), turn.Message))
		}
		prompt.WriteString("\n")
	}

	prompt.WriteString("Analyze the user's message and determine which tools to use to retrieve the requested information.\n\n")
	prompt.WriteString("Available tools:\n")

	// List all available tools
	for _, tool := range c.tools.GetAllTools() {
		prompt.WriteString(fmt.Sprintf("- %s: %s\n", tool.Name(), tool.Description()))

		// Include parameter information
		for _, param := range tool.Parameters() {
			required := ""
			if param.Required {
				required = " (required)"
			}
			prompt.WriteString(fmt.Sprintf("  - %s: %s%s\n", param.Name, param.Description, required))
		}
	}

	prompt.WriteString("\nUser message: ")
	prompt.WriteString(message)
	prompt.WriteString("\n\n")

	prompt.WriteString("CRITICAL: You MUST use tools to answer ANY question about entities.\n")
	prompt.WriteString("Even if you think you know the answer, you MUST use tools to retrieve the information.\n")
	prompt.WriteString("Do NOT rely on any knowledge from training - ONLY use tools.\n\n")

	prompt.WriteString("Respond with a JSON array of tool calls. Each tool call should have:\n")
	prompt.WriteString("- \"tool\": the tool name\n")
	prompt.WriteString("- \"args\": an object with the tool's arguments\n\n")
	prompt.WriteString("Example response:\n")
	prompt.WriteString(`[{"tool": "search_entities", "args": {"query": "douglas wilson"}}]`)
	prompt.WriteString("\n\nYour response (JSON only):")

	return prompt.String()
}

// parseToolCallsFromText attempts to extract tool calls from unstructured text
func (c *ChatInterface) parseToolCallsFromText(text string) []tools.ToolCall {
	// This is a simplified parser - in production, you'd want more sophisticated parsing
	var calls []tools.ToolCall

	// Look for common patterns
	lines := strings.SplitSeq(text, "\n")
	for line := range lines {
		line = strings.TrimSpace(line)

		// Look for patterns like "search for X" or "find Y"
		if strings.Contains(strings.ToLower(line), "search") {
			// Extract search query
			parts := strings.Split(line, "\"")
			if len(parts) >= 2 {
				calls = append(calls, tools.ToolCall{
					Tool: "search_entities",
					Args: map[string]any{
						"query": parts[1],
					},
				})
			}
		}

		// Look for "show X" or "display Y"
		if strings.Contains(strings.ToLower(line), "show") || strings.Contains(strings.ToLower(line), "display") {
			parts := strings.Split(line, "\"")
			if len(parts) >= 2 {
				calls = append(calls, tools.ToolCall{
					Tool: "read_entity",
					Args: map[string]any{
						"id": parts[1],
					},
				})
			}
		}
	}

	// If no specific tools found, try a general search
	if len(calls) == 0 {
		// Extract key terms from the message for search
		calls = append(calls, tools.ToolCall{
			Tool: "search_entities",
			Args: map[string]any{
				"query": text,
			},
		})
	}

	return calls
}

// formatResponse converts tool results into a natural language response
func (c *ChatInterface) formatResponse(ctx context.Context, originalMessage string, toolCalls []tools.ToolCall, results []tools.ToolResult) (string, error) {
	// Build context for the LLM
	var prompt strings.Builder

	prompt.WriteString("You are a knowledge graph assistant that ONLY reports information found in the local knowledge graph.\n")
	prompt.WriteString("IMPORTANT: Base your response ONLY on the tool results below. Do not add information from general knowledge.\n")
	prompt.WriteString("If the requested information is not in the tool results, say so clearly.\n\n")

	// Include conversation history for continuity
	if len(c.history) > 1 {
		prompt.WriteString("Recent conversation:\n")
		for i := max(0, len(c.history)-3); i < len(c.history); i++ {
			turn := c.history[i]
			switch turn.Role {
			case "user":
				prompt.WriteString(fmt.Sprintf("User: %s\n", turn.Message))
			case "assistant":
				// Show summary of previous response
				if len(turn.Message) > 200 {
					prompt.WriteString(fmt.Sprintf("You: %s...\n", turn.Message[:200]))
				} else {
					prompt.WriteString(fmt.Sprintf("You: %s\n", turn.Message))
				}
			}
		}
		prompt.WriteString("\n")
	}
	prompt.WriteString("The user asked: \"")
	prompt.WriteString(originalMessage)
	prompt.WriteString("\"\n\n")

	prompt.WriteString("You executed the following tools and got these results:\n\n")

	for i, call := range toolCalls {
		prompt.WriteString(fmt.Sprintf("Tool: %s\n", call.Tool))
		prompt.WriteString(fmt.Sprintf("Arguments: %v\n", call.Args))

		if i < len(results) {
			result := results[i]
			if result.Success {
				// Format the data nicely
				dataStr, _ := json.MarshalIndent(result.Data, "", "  ")
				prompt.WriteString(fmt.Sprintf("Result: %s\n", dataStr))
			} else {
				prompt.WriteString(fmt.Sprintf("Error: %s\n", result.Error))
			}
		}
		prompt.WriteString("\n")
	}

	prompt.WriteString("Based ONLY on these tool results, provide a helpful response to the user.\n")
	prompt.WriteString("Rules:\n")
	prompt.WriteString("1. Only mention information that appears in the tool results above\n")
	prompt.WriteString("2. If information is not found, explicitly say it's not in the knowledge graph\n")
	prompt.WriteString("3. Be concise but informative\n")
	prompt.WriteString("4. Format entity names, IDs, and important information clearly\n")
	prompt.WriteString("5. Do not add facts from your general knowledge - only report what's in the results\n\n")
	prompt.WriteString("Your response:")

	// Get LLM to format the response
	response, err := c.llm.Complete(ctx, prompt.String(), "")
	if err != nil {
		// Fallback to basic formatting
		return c.basicFormatResponse(toolCalls, results), nil
	}

	return response, nil
}

// basicFormatResponse provides a simple fallback formatting
func (c *ChatInterface) basicFormatResponse(toolCalls []tools.ToolCall, results []tools.ToolResult) string {
	var response strings.Builder

	for i, call := range toolCalls {
		if i < len(results) {
			result := results[i]
			if result.Success {
				switch call.Tool {
				case "search_entities":
					response.WriteString("Search results:\n")
					if matches, ok := result.Data.([]map[string]any); ok {
						for _, match := range matches {
							response.WriteString(fmt.Sprintf("- %s (%s)\n", match["title"], match["id"]))
						}
					}
				case "read_entity":
					response.WriteString("Entity details:\n")
					dataStr, _ := json.MarshalIndent(result.Data, "", "  ")
					response.Write(dataStr)
				default:
					response.WriteString(fmt.Sprintf("Tool %s completed successfully.\n", call.Tool))
				}
			} else {
				response.WriteString(fmt.Sprintf("Error: %s\n", result.Error))
			}
		}
	}

	return response.String()
}
