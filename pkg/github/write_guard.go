package github

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/sirupsen/logrus"
)

// checkRepoVisibility calls Repositories.Get() to determine if a repository is private.
// Returns (true, nil) if private, (false, nil) if public, (false, err) on any API error.
// Callers must treat any error as fail-closed (block the write).
//
// Do NOT use determineRepoAccessType() or RepoAccessCache for this purpose.
// Those functions check authenticated access success, not actual repo visibility.
// A PAT with access to a public repo returns RepoAccessPrivate from that function,
// which would incorrectly allow writes to public repos.
func checkRepoVisibility(ctx context.Context, getClient GetClientFn, owner, repo string) (bool, error) {
	client, err := getClient(ctx, owner)
	if err != nil {
		return false, fmt.Errorf("failed to get GitHub client: %w", err)
	}
	repoData, _, err := client.Repositories.Get(ctx, owner, repo)
	if err != nil {
		return false, fmt.Errorf("failed to get repository: %w", err)
	}
	return repoData.GetPrivate(), nil
}

// WritePrivateOnlyGuard wraps a write tool handler to enforce the GITHUB_WRITE_PRIVATE_ONLY
// policy. It extracts owner and repo from the MCP request, checks repository visibility
// via Repositories.Get(), and blocks the write if the repository is public or if
// visibility cannot be confirmed.
//
// On any error from the visibility check, it fails closed (blocks the write).
// If the repo is confirmed private, it delegates to the next handler unchanged.
//
// This guard is for standard write tools with owner+repo params.
// Use CreateRepositoryPrivateOnlyGuard for create_repository.
// Use ForkRepositoryPrivateOnlyGuard for fork_repository.
func WritePrivateOnlyGuard(getClient GetClientFn, tool mcp.Tool, handler server.ToolHandlerFunc) (mcp.Tool, server.ToolHandlerFunc) {
	return tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		owner, err := requiredParam[string](req, "owner")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		repo, err := requiredParam[string](req, "repo")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		isPrivate, err := checkRepoVisibility(ctx, getClient, owner, repo)
		if err != nil {
			logrus.Warnf("Write blocked: %s on %s/%s — visibility check failed: %v (GITHUB_WRITE_PRIVATE_ONLY)",
				tool.Name, owner, repo, err)
			return mcp.NewToolResultError(
				"Write blocked: unable to verify repository visibility. " +
					"Ensure your token has repo read access and try again.",
			), nil
		}
		if !isPrivate {
			logrus.Warnf("Write blocked: %s on public repo %s/%s (GITHUB_WRITE_PRIVATE_ONLY)",
				tool.Name, owner, repo)
			return mcp.NewToolResultError(fmt.Sprintf(
				"Write blocked: %s/%s is a public repository. "+
					"The server is configured with GITHUB_WRITE_PRIVATE_ONLY=true, which restricts "+
					"all write operations to private repositories only. "+
					"To proceed: use a private repository, or ask the administrator to unset GITHUB_WRITE_PRIVATE_ONLY.",
				owner, repo,
			)), nil
		}

		return handler(ctx, req)
	}
}

// CreateRepositoryPrivateOnlyGuard wraps the create_repository handler to enforce
// GITHUB_WRITE_PRIVATE_ONLY. Performs a pre-flight parameter check (no API call)
// because there is no existing repository to query for visibility.
//
// If the private parameter is false or absent (defaults to false via OptionalParam[bool]),
// the call is blocked immediately. If private=true, the call proceeds to the wrapped handler.
//
// Never silently overrides private=false to private=true.
func CreateRepositoryPrivateOnlyGuard(tool mcp.Tool, handler server.ToolHandlerFunc) (mcp.Tool, server.ToolHandlerFunc) {
	return tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		private, err := OptionalParam[bool](req, "private")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if !private {
			return mcp.NewToolResultError(
				"Write blocked: create_repository requires private=true when GITHUB_WRITE_PRIVATE_ONLY is set. " +
					"Set private=true to create a private repository.",
			), nil
		}
		return handler(ctx, req)
	}
}

// ForkRepositoryPrivateOnlyGuard replaces the fork_repository handler entirely when
// GITHUB_WRITE_PRIVATE_ONLY is set. GitHub's CreateFork API has no visibility parameter —
// fork visibility is determined by source repo visibility and the user's GitHub plan.
// Since we cannot guarantee the fork will be private, we block entirely.
//
// The original handler is accepted as a parameter for signature consistency but is never called.
func ForkRepositoryPrivateOnlyGuard(tool mcp.Tool, _ server.ToolHandlerFunc) (mcp.Tool, server.ToolHandlerFunc) {
	return tool, func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultError(
			"Write blocked: fork_repository cannot guarantee the fork will be private when " +
				"GITHUB_WRITE_PRIVATE_ONLY is set. GitHub's fork API does not expose a visibility parameter. " +
				"To create a private fork, use the GitHub web UI or API directly.",
		), nil
	}
}
