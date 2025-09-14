package mcp

import (
	"fmt"
	"log"
	"os"

	mcp "github.com/metoro-io/mcp-golang"
	"github.com/metoro-io/mcp-golang/transport/stdio"
	"silvia/internal/graph"
	"silvia/internal/llm"
	"silvia/internal/operations"
	"silvia/internal/sources"
)

// Server exposes Silvia's functionality using the operations layer
type Server struct {
	ops *operations.Operations
}

// RunMCPServer runs the MCP server using the operations layer
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

	log.Println("Starting Silvia MCP server v2 with operations layer...")

	// Initialize data directory
	dataDir := os.Getenv("SILVIA_DATA_DIR")
	if dataDir == "" {
		dataDir = "./data"
	}

	// Initialize graph manager
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

	// Initialize sources manager
	sourcesManager := sources.NewManager()

	// Create operations layer
	ops := operations.New(graphManager, llmClient, sourcesManager, dataDir)
	log.Println("Operations layer initialized")

	// Create the MCP server with stdio transport
	server := mcp.NewServer(stdio.NewStdioServerTransport())

	// Register all operations-based tools with the MCP server
	log.Println("Registering operations-based tools with MCP server...")
	if err := RegisterOperationsTools(server, ops); err != nil {
		return fmt.Errorf("failed to register operations tools: %w", err)
	}

	// Serve MCP requests
	log.Println("MCP server v2 ready, serving requests...")
	if err := server.Serve(); err != nil {
		return fmt.Errorf("server error: %w", err)
	}

	// Block forever - the server runs in background goroutines
	select {}
}
