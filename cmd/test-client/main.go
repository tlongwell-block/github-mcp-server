package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

func main() {
	// Get authentication credentials from environment variables
	appID := os.Getenv("GITHUB_APP_ID")
	installationID := os.Getenv("GITHUB_INSTALLATION_ID")
	privateKeyPath := os.Getenv("GITHUB_PRIVATE_KEY_FILE_PATH")
	privateKey := os.Getenv("GITHUB_PRIVATE_KEY")
	token := os.Getenv("GITHUB_PERSONAL_ACCESS_TOKEN")

	// Check if we have authentication credentials
	if appID == "" || installationID == "" || (privateKeyPath == "" && privateKey == "") {
		if token == "" {
			fmt.Println("Error: No authentication credentials provided.")
			fmt.Println("Please set either:")
			fmt.Println("  - GITHUB_APP_ID, GITHUB_INSTALLATION_ID, and either GITHUB_PRIVATE_KEY_FILE_PATH or GITHUB_PRIVATE_KEY")
			fmt.Println("  - GITHUB_PERSONAL_ACCESS_TOKEN")
			os.Exit(1)
		}
	}

	// Prepare environment variables for the client
	var env []string
	if appID != "" && installationID != "" {
		env = append(env, "GITHUB_APP_ID="+appID)
		env = append(env, "GITHUB_INSTALLATION_ID="+installationID)
		if privateKeyPath != "" {
			env = append(env, "GITHUB_PRIVATE_KEY_FILE_PATH="+privateKeyPath)
		} else if privateKey != "" {
			env = append(env, "GITHUB_PRIVATE_KEY="+privateKey)
		}
	} else if token != "" {
		env = append(env, "GITHUB_PERSONAL_ACCESS_TOKEN="+token)
	}

	// Create the client
	mcpClient, err := client.NewStdioMCPClient(
		"./github-mcp-server",
		env,
		"stdio",
	)
	if err != nil {
		fmt.Printf("Error creating client: %v\n", err)
		os.Exit(1)
	}
	defer mcpClient.Close()

	// Initialize the client
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	request := mcp.InitializeRequest{}
	request.Params.ProtocolVersion = "2025-03-26"
	request.Params.ClientInfo = mcp.Implementation{
		Name:    "test-client",
		Version: "0.0.1",
	}

	result, err := mcpClient.Initialize(ctx, request)
	if err != nil {
		fmt.Printf("Error initializing client: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Server info: %s %s\n", result.ServerInfo.Name, result.ServerInfo.Version)

	// Try to access a public repository
	getFileContentsRequest := mcp.CallToolRequest{}
	getFileContentsRequest.Params.Name = "get_file_contents"
	getFileContentsRequest.Params.Arguments = map[string]any{
		"owner": "github",
		"repo":  "github-mcp-server",
		"path":  "README.md",
	}

	fmt.Println("Getting file contents for github/github-mcp-server/README.md...")
	resp, err := mcpClient.CallTool(ctx, getFileContentsRequest)
	if err != nil {
		fmt.Printf("Error calling get_file_contents: %v\n", err)
		os.Exit(1)
	}

	if resp.IsError {
		fmt.Printf("Error response: IsError=true\n")
		os.Exit(1)
	}

	fmt.Printf("Got response with %d content items\n", len(resp.Content))
	if len(resp.Content) > 0 {
		textContent, ok := resp.Content[0].(mcp.TextContent)
		if ok {
			fmt.Printf("Content type: %s\n", textContent.Type)
			fmt.Printf("Content length: %d bytes\n", len(textContent.Text))
		}
	}

	// Try to access a private repository
	getFileContentsRequest = mcp.CallToolRequest{}
	getFileContentsRequest.Params.Name = "get_file_contents"
	getFileContentsRequest.Params.Arguments = map[string]any{
		"owner": "goose-slackbot",
		"repo":  "goose-slackbot",
		"path":  "README.md",
	}

	fmt.Println("\nGetting file contents for goose-slackbot/goose-slackbot/README.md...")
	resp, err = mcpClient.CallTool(ctx, getFileContentsRequest)
	if err != nil {
		fmt.Printf("Error calling get_file_contents: %v\n", err)
		os.Exit(1)
	}

	if resp.IsError {
		fmt.Printf("Error response: IsError=true\n")
		os.Exit(1)
	}

	fmt.Printf("Got response with %d content items\n", len(resp.Content))
	if len(resp.Content) > 0 {
		textContent, ok := resp.Content[0].(mcp.TextContent)
		if ok {
			fmt.Printf("Content type: %s\n", textContent.Type)
			fmt.Printf("Content length: %d bytes\n", len(textContent.Text))
		}
	}
}
