package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"silvia/internal/bsky"
	"silvia/internal/cli"
	"silvia/internal/graph"
	"silvia/internal/llm"
	"silvia/internal/mcp"
	"silvia/internal/server"
)

func main() {
	var (
		help          bool
		bskyHandle    string
		bskyPassword  string
		openrouterKey string
		dataDir       string
		serverPort    int
		serverToken   string
		noServer      bool
		debug         bool
		mcpMode       bool
	)

	flag.BoolVar(&help, "help", false, "Show help message")
	flag.BoolVar(&help, "h", false, "Show help message (shorthand)")
	flag.StringVar(&bskyHandle, "bsky-handle", os.Getenv("BSKY_HANDLE"), "Bluesky handle (can also use BSKY_HANDLE env var)")
	flag.StringVar(&bskyPassword, "bsky-password", os.Getenv("BSKY_PASSWORD"), "Bluesky app password (can also use BSKY_PASSWORD env var)")
	flag.StringVar(&openrouterKey, "openrouter-key", os.Getenv("OPENROUTER_API_KEY"), "OpenRouter API key (can also use OPENROUTER_API_KEY env var)")
	flag.StringVar(&dataDir, "data", "./data", "Data directory for storing the knowledge graph")
	flag.IntVar(&serverPort, "port", 8765, "Port for browser extension API server")
	flag.StringVar(&serverToken, "token", os.Getenv("SILVIA_TOKEN"), "Optional auth token for extension API (can also use SILVIA_TOKEN env var)")
	flag.BoolVar(&noServer, "no-server", false, "Disable the extension API server")
	flag.BoolVar(&debug, "debug", false, "Enable debug output for troubleshooting")
	flag.BoolVar(&mcpMode, "mcp", false, "Run as MCP server for AI assistants (requires stdio connection)")
	flag.Parse()

	if help {
		fmt.Println("silvia - Knowledge Graph Explorer")
		fmt.Println()
		fmt.Println("Usage:")
		fmt.Printf("  %s [flags]\n", os.Args[0])
		fmt.Println()
		fmt.Println("Flags:")
		flag.PrintDefaults()
		fmt.Println()
		fmt.Println("Environment Variables:")
		fmt.Println("  BSKY_HANDLE          Bluesky handle")
		fmt.Println("  BSKY_PASSWORD        Bluesky app password")
		fmt.Println("  OPENROUTER_API_KEY   OpenRouter API key")
		fmt.Println("  SILVIA_TOKEN         Optional auth token for extension API")
		fmt.Println()
		fmt.Println("MCP Server Mode:")
		fmt.Println("  Run with -mcp flag to start as an MCP server for AI assistants.")
		fmt.Println("  This mode requires stdin/stdout to be connected (not a terminal).")
		fmt.Println("  Example: silvia -mcp < /dev/null")
		os.Exit(0)
	}

	// If MCP mode is requested, run as MCP server
	if mcpMode {
		if err := mcp.RunMCPServer(); err != nil {
			log.Fatalf("MCP server error: %v", err)
		}
		return
	}

	// Initialize graph manager
	graphManager := graph.NewManager(dataDir)
	if err := graphManager.InitializeDirectories(); err != nil {
		log.Fatalf("Failed to initialize directories: %v", err)
	}

	// Initialize Bluesky client (optional)
	var bskyClient *bsky.Client
	if bskyHandle != "" && bskyPassword != "" {
		client, err := bsky.NewClient(bskyHandle, bskyPassword)
		if err != nil {
			log.Printf("Warning: Failed to initialize Bluesky client: %v", err)
		} else {
			bskyClient = client
			log.Printf("Bluesky client initialized for handle: %s", bskyHandle)
		}
	}

	// Initialize OpenRouter client (required)
	if openrouterKey == "" {
		log.Fatal("Error: OPENROUTER_API_KEY environment variable is required")
	}
	llmClient := llm.NewClient(openrouterKey)
	log.Println("OpenRouter client initialized")

	// Store clients in context for later use
	ctx := context.Background()
	if bskyClient != nil {
		ctx = context.WithValue(ctx, "bsky", bskyClient)
	}

	// Initialize CLI
	cliInterface := cli.NewCLI(graphManager, llmClient)

	// Enable debug mode if requested
	if debug {
		log.Println("Debug mode enabled")
		cliInterface.SetDebug(true)
	}

	// Load queue from disk
	queuePath := filepath.Join(dataDir, ".silvia", "queue.json")
	if err := cliInterface.LoadQueue(queuePath); err != nil {
		log.Printf("Warning: Failed to load queue: %v", err)
	}

	// Start extension API server if enabled
	if !noServer {
		ops := cliInterface.GetOperations()
		if ops == nil {
			log.Fatal("Operations layer not available")
		}

		apiServer := server.NewServer(serverPort, serverToken, ops)
		go func() {
			if err := apiServer.Start(); err != nil && err != http.ErrServerClosed {
				log.Printf("API server error: %v", err)
			}
		}()
		log.Printf("API server started on port %d", serverPort)

		// Ensure server stops on exit
		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := apiServer.Stop(shutdownCtx); err != nil {
				log.Printf("Error stopping server: %v", err)
			}
		}()
	} else {
		log.Println("Extension API server disabled")
	}

	// Run interactive CLI
	if err := cliInterface.Run(ctx); err != nil {
		log.Fatalf("CLI error: %v", err)
	}
}
