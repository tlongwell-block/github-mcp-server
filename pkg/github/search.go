package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v69/github"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// MinimalCodeResult is a compact representation of a code search result
// optimized for LLM consumption.
type MinimalCodeResult struct {
	Name        string   `json:"name"`
	Path        string   `json:"path"`
	HTMLURL     string   `json:"html_url"`
	Repository  string   `json:"repository"`             // "owner/repo" format
	TextMatches []string `json:"text_matches,omitempty"` // code fragments
}

// MinimalCodeSearchResult is a compact representation of a code search response.
type MinimalCodeSearchResult struct {
	TotalCount        int                 `json:"total_count"`
	IncompleteResults bool                `json:"incomplete_results"`
	Items             []MinimalCodeResult `json:"items"`
}

// SearchRepositories creates a tool to search for GitHub repositories.
func SearchRepositories(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("search_repositories",
			mcp.WithDescription(t("TOOL_SEARCH_REPOSITORIES_DESCRIPTION", `Search for GitHub repositories by name, description, topics, or other metadata.

Useful for discovering projects, finding repos by topic, or locating specific repositories.

Examples:
- "machine learning" language:python stars:>100
- topic:kubernetes org:myorg
- "payment" in:name org:myorg`)),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_SEARCH_REPOSITORIES_USER_TITLE", "Search repositories"),
				ReadOnlyHint: toBoolPtr(true),
			}),
			mcp.WithString("query",
				mcp.Required(),
				mcp.Description("Search query"),
			),
			WithPagination(),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			query, err := requiredParam[string](request, "query")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			pagination, err := OptionalPaginationParams(request)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			repoQuery := extractRepoFromQuery(query)
			owner := ""
			if repoQuery.owner != "" && repoQuery.repo != "" {
				owner = repoQuery.owner
				ctx = WithRepoContext(ctx, repoQuery.owner, repoQuery.repo)
			} else if org := extractOrgFromQuery(query); org != "" {
				owner = org
			}

			opts := &github.SearchOptions{
				ListOptions: github.ListOptions{
					Page:    pagination.page,
					PerPage: pagination.perPage,
				},
			}

			client, err := getClient(ctx, owner)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}
			result, resp, err := client.Search.Repositories(ctx, query, opts)
			if err != nil {
				return nil, fmt.Errorf("failed to search repositories: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != 200 {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to search repositories: %s", string(body))), nil
			}

			r, err := json.Marshal(result)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// SearchCode creates a tool to search for code across GitHub repositories.
func SearchCode(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("search_code",
			mcp.WithDescription(t("TOOL_SEARCH_CODE_DESCRIPTION", `Search for code across GitHub repositories using the REST API.

IMPORTANT: This tool uses GitHub's legacy code search API. Only the qualifiers listed below are supported.
Do NOT use: content:, symbol:, is:, NOT, OR, parentheses, /regex/, or glob patterns — they will silently fail.

Supported qualifiers:
- org:NAME or user:NAME — scope to an organization (ALWAYS include this for broad searches)
- repo:OWNER/NAME — scope to a specific repository
- language:NAME — filter by programming language
- path:DIRECTORY — filter by directory path (basic only, no glob)
- filename:NAME — find files by name
- extension:EXT — find files by extension
- in:file or in:path — search file contents vs file paths
- size:N — filter by file size (e.g. size:>1000)
- "exact phrase" — quoted exact string match
- fork:true — include results from forked repositories
- Multiple terms are AND'd automatically

Rate limit: 10 searches per minute. Plan your query carefully before searching.

Examples:
- "class AuthHandler" language:python org:myorg
- filename:Dockerfile org:myorg
- "import express" extension:ts repo:owner/repo
- path:src/api "middleware" language:go org:myorg

After finding files, use get_file_contents to read the full source code.`)),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_SEARCH_CODE_USER_TITLE", "Search code"),
				ReadOnlyHint: toBoolPtr(true),
			}),
			mcp.WithString("query",
				mcp.Required(),
				mcp.Description("Search query using GitHub code search qualifiers. Always scope with org: or repo: for best results."),
			),
			mcp.WithString("sort",
				mcp.Description("Sort field ('indexed' only)"),
			),
			mcp.WithString("order",
				mcp.Description("Sort order"),
				mcp.Enum("asc", "desc"),
			),
			WithPagination(),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			query, err := requiredParam[string](request, "query")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			sort, err := OptionalParam[string](request, "sort")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			order, err := OptionalParam[string](request, "order")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			pagination, err := OptionalPaginationParams(request)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			repoQuery := extractRepoFromQuery(query)
			owner := ""
			if repoQuery.owner != "" && repoQuery.repo != "" {
				owner = repoQuery.owner
				ctx = WithRepoContext(ctx, repoQuery.owner, repoQuery.repo)
			} else if org := extractOrgFromQuery(query); org != "" {
				owner = org
			}

			opts := &github.SearchOptions{
				Sort:      sort,
				Order:     order,
				TextMatch: true,
				ListOptions: github.ListOptions{
					PerPage: pagination.perPage,
					Page:    pagination.page,
				},
			}

			client, err := getClient(ctx, owner)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			result, resp, err := client.Search.Code(ctx, query, opts)
			if err != nil {
				return nil, fmt.Errorf("failed to search code: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != 200 {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to search code: %s", string(body))), nil
			}

			// Format as minimal results for LLM consumption
			minimalResults := make([]MinimalCodeResult, 0, len(result.CodeResults))
			for _, cr := range result.CodeResults {
				mr := MinimalCodeResult{
					Name:       cr.GetName(),
					Path:       cr.GetPath(),
					HTMLURL:    cr.GetHTMLURL(),
					Repository: cr.GetRepository().GetFullName(),
				}
				// Extract text match fragments
				for _, tm := range cr.TextMatches {
					if tm.Fragment != nil {
						mr.TextMatches = append(mr.TextMatches, *tm.Fragment)
					}
				}
				minimalResults = append(minimalResults, mr)
			}

			minimalResult := MinimalCodeSearchResult{
				TotalCount:        result.GetTotal(),
				IncompleteResults: result.GetIncompleteResults(),
				Items:             minimalResults,
			}

			r, err := json.Marshal(minimalResult)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// SearchUsers creates a tool to search for GitHub users.
func SearchUsers(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("search_users",
			mcp.WithDescription(t("TOOL_SEARCH_USERS_DESCRIPTION", `Search for GitHub users by username, name, location, or other profile information.

Examples:
- "john" location:seattle
- followers:>100 language:go`)),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_SEARCH_USERS_USER_TITLE", "Search users"),
				ReadOnlyHint: toBoolPtr(true),
			}),
			mcp.WithString("query",
				mcp.Required(),
				mcp.Description("Search query for GitHub users. Examples: 'location:seattle', 'followers:>100'."),
			),
			mcp.WithString("sort",
				mcp.Description("Sort field by category"),
				mcp.Enum("followers", "repositories", "joined"),
			),
			mcp.WithString("order",
				mcp.Description("Sort order"),
				mcp.Enum("asc", "desc"),
			),
			WithPagination(),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			query, err := requiredParam[string](request, "query")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			sort, err := OptionalParam[string](request, "sort")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			order, err := OptionalParam[string](request, "order")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			pagination, err := OptionalPaginationParams(request)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			opts := &github.SearchOptions{
				Sort:  sort,
				Order: order,
				ListOptions: github.ListOptions{
					PerPage: pagination.perPage,
					Page:    pagination.page,
				},
			}

			owner := extractOrgFromQuery(query)

			client, err := getClient(ctx, owner)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			result, resp, err := client.Search.Users(ctx, query, opts)
			if err != nil {
				return nil, fmt.Errorf("failed to search users: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != 200 {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to search users: %s", string(body))), nil
			}

			r, err := json.Marshal(result)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}
