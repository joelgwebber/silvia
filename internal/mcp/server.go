package mcp

import (
	"fmt"
	"log"
	"os"

	mcp "github.com/metoro-io/mcp-golang"
	"github.com/metoro-io/mcp-golang/transport/stdio"
	"silvia/internal/graph"
	"silvia/internal/llm"
)

// DirectMCPServer exposes Silvia's functionality directly as MCP tools
type MCPServer struct {
	graphManager *graph.Manager
	llmClient    *llm.Client
}

// RunDirectMCPServer runs the MCP server with direct graph access
func RunMCPServer() error {
	// Set up logging to stderr so it doesn't interfere with stdio protocol
	log.SetOutput(os.Stderr)
	log.SetPrefix("[SILVIA-MCP] ")
	log.SetFlags(log.Ltime | log.Lmicroseconds)

	// Check if we're in MCP mode (stdio connected)
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) != 0 {
		return fmt.Errorf("MCP server mode requires stdin/stdout to be connected (not a terminal)")
	}

	log.Println("Starting Silvia Direct MCP server...")

	// Initialize graph manager
	dataDir := os.Getenv("SILVIA_DATA_DIR")
	if dataDir == "" {
		dataDir = "./data"
	}
	graphManager := graph.NewManager(dataDir)
	if err := graphManager.InitializeDirectories(); err != nil {
		return fmt.Errorf("failed to initialize directories: %w", err)
	}

	// Initialize LLM client if API key is available
	var llmClient *llm.Client
	openrouterKey := os.Getenv("OPENROUTER_API_KEY")
	if openrouterKey != "" {
		llmClient = llm.NewClient(openrouterKey)
		log.Println("LLM client initialized")
	} else {
		log.Println("Warning: No OPENROUTER_API_KEY found, LLM features disabled")
	}

	// Create the MCP server with stdio transport
	server := mcp.NewServer(stdio.NewStdioServerTransport())

	// Register graph tools
	if err := registerEntityTools(server, graphManager, llmClient); err != nil {
		return fmt.Errorf("failed to register tools: %w", err)
	}

	// Register queue tools
	if err := registerQueueTools(server, graphManager, llmClient); err != nil {
		return fmt.Errorf("failed to register queue tools: %w", err)
	}

	// Serve MCP requests
	log.Println("MCP server ready, serving requests...")
	if err := server.Serve(); err != nil {
		return fmt.Errorf("server error: %w", err)
	}

	// Block forever - the server runs in background goroutines
	select {}
}
