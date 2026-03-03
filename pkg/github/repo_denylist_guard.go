package github

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/sirupsen/logrus"
)

// RepoDenylistGuard wraps a tool handler to enforce GITHUB_REPO_DENYLIST.
// It extracts owner and repo from the MCP request and blocks both reads and
// writes if the repository matches the denylist. Runs before other guards
// (e.g. WritePrivateOnlyGuard) so denied repos are rejected without making
// any GitHub API calls.
func RepoDenylistGuard(denylist *RepoDenylist, tool mcp.Tool, handler server.ToolHandlerFunc) (mcp.Tool, server.ToolHandlerFunc) {
	return tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		owner, err := requiredParam[string](req, "owner")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		repo, err := requiredParam[string](req, "repo")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if denylist.IsDenied(owner, repo) {
			logrus.Warnf("Access blocked: %s on denied repo %s/%s (GITHUB_REPO_DENYLIST)", tool.Name, owner, repo)
			return mcp.NewToolResultError(fmt.Sprintf(
				"Access blocked: %s/%s is on the repository denylist. "+
					"The server is configured with GITHUB_REPO_DENYLIST, which restricts "+
					"all operations on specified repositories. "+
					"Contact the administrator if you believe this is an error.",
				owner, repo,
			)), nil
		}

		return handler(ctx, req)
	}
}

// SearchDenylistGuard wraps a search tool handler to block queries that
// explicitly target a denied repository or organization. queryParam is the
// name of the string parameter holding the search query (e.g. "query" or "q").
//
// Uses extractRepoFromQuery and extractOrgFromQuery from query_helpers.go to
// detect "repo:owner/repo" and "org:owner" / "user:owner" qualifiers.
// Unscoped searches that happen to return results from denied repos are not
// filtered — any follow-up tool call (e.g. get_file_contents) will be blocked
// by RepoDenylistGuard.
func SearchDenylistGuard(denylist *RepoDenylist, queryParam string, tool mcp.Tool, handler server.ToolHandlerFunc) (mcp.Tool, server.ToolHandlerFunc) {
	return tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query, err := requiredParam[string](req, queryParam)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Block if any "repo:owner/repo" qualifier in the query targets a denied repo.
		// All qualifiers are checked to prevent bypass via multiple repo: qualifiers
		// (e.g. "repo:allowed/repo repo:denied/repo").
		for _, info := range extractAllReposFromQuery(query) {
			if denylist.IsDenied(info.owner, info.repo) {
				logrus.Warnf("Search blocked: %s targeting denied repo %s/%s (GITHUB_REPO_DENYLIST)",
					tool.Name, info.owner, info.repo)
				return mcp.NewToolResultError(fmt.Sprintf(
					"Search blocked: the query targets %s/%s, which is on the repository denylist. "+
						"Contact the administrator if you believe this is an error.",
					info.owner, info.repo,
				)), nil
			}
		}

		// Block if the query targets a denied org via "org:owner" or "user:owner"
		if org := extractOrgFromQuery(query); org != "" && denylist.IsOrgDenied(org) {
			logrus.Warnf("Search blocked: %s targeting denied org %s (GITHUB_REPO_DENYLIST)", tool.Name, org)
			return mcp.NewToolResultError(fmt.Sprintf(
				"Search blocked: the query targets organization %s, all of whose repositories "+
					"are on the denylist. Contact the administrator if you believe this is an error.",
				org,
			)), nil
		}

		return handler(ctx, req)
	}
}

// DenylistResourceGuard wraps a resource template handler to enforce
// GITHUB_REPO_DENYLIST for repo:// URI resource requests. Extracts owner
// and repo from the resource request arguments (same as RepositoryResourceContentsHandler).
func DenylistResourceGuard(denylist *RepoDenylist, handler server.ResourceTemplateHandlerFunc) server.ResourceTemplateHandlerFunc {
	return func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		o, ok := request.Params.Arguments["owner"].([]string)
		if ok && len(o) > 0 {
			r, ok := request.Params.Arguments["repo"].([]string)
			if ok && len(r) > 0 {
				owner, repo := o[0], r[0]
				if denylist.IsDenied(owner, repo) {
					logrus.Warnf("Resource access blocked: %s/%s is on the denylist (GITHUB_REPO_DENYLIST)", owner, repo)
					return nil, fmt.Errorf(
						"Access blocked: %s/%s is on the repository denylist. "+
							"Contact the administrator if you believe this is an error.",
						owner, repo,
					)
				}
			}
		}
		return handler(ctx, request)
	}
}
