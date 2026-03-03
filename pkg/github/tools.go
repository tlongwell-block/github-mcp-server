package github

import (
	"context"
	"fmt"

	"github.com/github/github-mcp-server/pkg/toolsets"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v69/github"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/shurcooL/githubv4"
)

type GetClientFn func(ctx context.Context, owner string) (*github.Client, error)
type GetGQLClientFn func(ctx context.Context, owner string) (*githubv4.Client, error)

var DefaultTools = []string{"all"}

func InitToolsets(passedToolsets []string, readOnly bool, writePrivateOnly bool, denylist *RepoDenylist, getClient GetClientFn, getGQLClient GetGQLClientFn, t translations.TranslationHelperFunc) (*toolsets.ToolsetGroup, error) {
	// Parse toolset configurations from the passed toolsets
	configs, err := toolsets.ParseToolsetConfigFromSlice(passedToolsets)
	if err != nil {
		return nil, fmt.Errorf("failed to parse toolset configuration: %w", err)
	}

	return InitToolsetsWithConfig(configs, readOnly, writePrivateOnly, denylist, getClient, getGQLClient, t)
}

func InitToolsetsWithConfig(configs []toolsets.ToolsetConfig, readOnly bool, writePrivateOnly bool, denylist *RepoDenylist, getClient GetClientFn, getGQLClient GetGQLClientFn, t translations.TranslationHelperFunc) (*toolsets.ToolsetGroup, error) {
	// Create a new toolset group
	tsg := toolsets.NewToolsetGroup(readOnly)

	// Helper functions to conditionally wrap tool handlers with guards.
	// When the relevant feature is disabled, these are no-ops returning the tool and handler unchanged.

	// guardDenylist wraps any tool with owner+repo params to block denied repos (reads and writes).
	// Runs outermost so denied repos are rejected before any other guard or API call.
	guardDenylist := func(tool mcp.Tool, handler server.ToolHandlerFunc) (mcp.Tool, server.ToolHandlerFunc) {
		if denylist != nil && !denylist.IsEmpty() {
			return RepoDenylistGuard(denylist, tool, handler)
		}
		return tool, handler
	}
	// guardSearchDenylist wraps search tools to block queries explicitly targeting denied repos/orgs.
	guardSearchDenylist := func(queryParam string) func(mcp.Tool, server.ToolHandlerFunc) (mcp.Tool, server.ToolHandlerFunc) {
		return func(tool mcp.Tool, handler server.ToolHandlerFunc) (mcp.Tool, server.ToolHandlerFunc) {
			if denylist != nil && !denylist.IsEmpty() {
				return SearchDenylistGuard(denylist, queryParam, tool, handler)
			}
			return tool, handler
		}
	}
	guardWrite := func(tool mcp.Tool, handler server.ToolHandlerFunc) (mcp.Tool, server.ToolHandlerFunc) {
		if writePrivateOnly {
			return WritePrivateOnlyGuard(getClient, tool, handler)
		}
		return tool, handler
	}
	guardCreate := func(tool mcp.Tool, handler server.ToolHandlerFunc) (mcp.Tool, server.ToolHandlerFunc) {
		if writePrivateOnly {
			return CreateRepositoryPrivateOnlyGuard(tool, handler)
		}
		return tool, handler
	}
	guardFork := func(tool mcp.Tool, handler server.ToolHandlerFunc) (mcp.Tool, server.ToolHandlerFunc) {
		if writePrivateOnly {
			return ForkRepositoryPrivateOnlyGuard(tool, handler)
		}
		return tool, handler
	}

	// Define all available features with their default state (disabled)
	// Create toolsets
	repos := toolsets.NewToolset("repos", "GitHub Repository related tools").
		AddReadTools(
			toolsets.NewServerTool(guardSearchDenylist("query")(SearchRepositories(getClient, t))),
			toolsets.NewServerTool(guardDenylist(GetFileContents(getClient, t))),
			toolsets.NewServerTool(guardDenylist(ListCommits(getClient, t))),
			toolsets.NewServerTool(guardSearchDenylist("q")(guardDenylist(SearchCode(getClient, t)))),
			toolsets.NewServerTool(guardDenylist(GetCommit(getClient, t))),
			toolsets.NewServerTool(guardDenylist(ListBranches(getClient, t))),
			toolsets.NewServerTool(guardDenylist(ListTags(getClient, t))),
			toolsets.NewServerTool(guardDenylist(GetTag(getClient, t))),
		).
		AddWriteTools(
			toolsets.NewServerTool(guardDenylist(guardWrite(CreateOrUpdateFile(getClient, t)))),
			toolsets.NewServerTool(guardCreate(CreateRepository(getClient, t))),
			toolsets.NewServerTool(guardDenylist(guardFork(ForkRepository(getClient, t)))),
			toolsets.NewServerTool(guardDenylist(guardWrite(CreateBranch(getClient, t)))),
			toolsets.NewServerTool(guardDenylist(guardWrite(PushFiles(getClient, t)))),
			toolsets.NewServerTool(guardDenylist(guardWrite(DeleteFile(getClient, t)))),
		)
	issues := toolsets.NewToolset("issues", "GitHub Issues related tools").
		AddReadTools(
			toolsets.NewServerTool(guardDenylist(GetIssue(getClient, t))),
			toolsets.NewServerTool(guardSearchDenylist("q")(SearchIssues(getClient, t))),
			toolsets.NewServerTool(guardDenylist(ListIssues(getClient, t))),
			toolsets.NewServerTool(guardDenylist(GetIssueComments(getClient, t))),
		).
		AddWriteTools(
			toolsets.NewServerTool(guardDenylist(guardWrite(CreateIssue(getClient, t)))),
			toolsets.NewServerTool(guardDenylist(guardWrite(AddIssueComment(getClient, t)))),
			toolsets.NewServerTool(guardDenylist(guardWrite(UpdateIssue(getClient, t)))),
		)
	users := toolsets.NewToolset("users", "GitHub User related tools").
		AddReadTools(
			toolsets.NewServerTool(guardSearchDenylist("q")(SearchUsers(getClient, t))),
		)
	pullRequests := toolsets.NewToolset("pull_requests", "GitHub Pull Request related tools").
		AddReadTools(
			toolsets.NewServerTool(guardDenylist(GetPullRequest(getClient, t))),
			toolsets.NewServerTool(guardDenylist(ListPullRequests(getClient, t))),
			toolsets.NewServerTool(guardDenylist(GetPullRequestFiles(getClient, t))),
			toolsets.NewServerTool(guardDenylist(GetPullRequestStatus(getClient, t))),
			toolsets.NewServerTool(guardDenylist(GetPullRequestComments(getClient, t))),
			toolsets.NewServerTool(guardDenylist(GetPullRequestReviews(getClient, t))),
			toolsets.NewServerTool(guardDenylist(GetPullRequestDiff(getClient, t))),
		).
		AddWriteTools(
			toolsets.NewServerTool(guardDenylist(guardWrite(MergePullRequest(getClient, t)))),
			toolsets.NewServerTool(guardDenylist(guardWrite(UpdatePullRequestBranch(getClient, t)))),
			toolsets.NewServerTool(guardDenylist(guardWrite(CreatePullRequest(getClient, t)))),
			toolsets.NewServerTool(guardDenylist(guardWrite(UpdatePullRequest(getClient, t)))),
			toolsets.NewServerTool(guardDenylist(guardWrite(RequestCopilotReview(getClient, t)))),

			// Reviews
			toolsets.NewServerTool(guardDenylist(guardWrite(CreateAndSubmitPullRequestReview(getGQLClient, t)))),
			toolsets.NewServerTool(guardDenylist(guardWrite(CreatePendingPullRequestReview(getGQLClient, t)))),
			toolsets.NewServerTool(guardDenylist(guardWrite(AddPullRequestReviewCommentToPendingReview(getGQLClient, t)))),
			toolsets.NewServerTool(guardDenylist(guardWrite(SubmitPendingPullRequestReview(getGQLClient, t)))),
			toolsets.NewServerTool(guardDenylist(guardWrite(DeletePendingPullRequestReview(getGQLClient, t)))),
		)
	codeSecurity := toolsets.NewToolset("code_security", "Code security related tools, such as GitHub Code Scanning").
		AddReadTools(
			toolsets.NewServerTool(guardDenylist(GetCodeScanningAlert(getClient, t))),
			toolsets.NewServerTool(guardDenylist(ListCodeScanningAlerts(getClient, t))),
		)
	secretProtection := toolsets.NewToolset("secret_protection", "Secret protection related tools, such as GitHub Secret Scanning").
		AddReadTools(
			toolsets.NewServerTool(guardDenylist(GetSecretScanningAlert(getClient, t))),
			toolsets.NewServerTool(guardDenylist(ListSecretScanningAlerts(getClient, t))),
		)
	// Keep experiments alive so the system doesn't error out when it's always enabled
	experiments := toolsets.NewToolset("experiments", "Experimental features that are not considered stable yet")

	// Create context toolset (always available)
	context := toolsets.NewToolset("context", "Tools that provide context about the current user and GitHub context you are operating in").
		AddReadTools(
			toolsets.NewServerTool(GetMe(getClient, t)),
		)

	// Add toolsets to the group
	tsg.AddToolset(repos)
	tsg.AddToolset(issues)
	tsg.AddToolset(users)
	tsg.AddToolset(pullRequests)
	tsg.AddToolset(codeSecurity)
	tsg.AddToolset(secretProtection)
	tsg.AddToolset(experiments)
	tsg.AddToolset(context)

	// Enable the requested toolsets with their configurations
	if err := tsg.EnableToolsetsWithConfig(configs); err != nil {
		return nil, err
	}

	return tsg, nil
}

func InitContextToolset(getClient GetClientFn, t translations.TranslationHelperFunc) *toolsets.Toolset {
	// Create a new context toolset
	contextTools := toolsets.NewToolset("context", "Tools that provide context about the current user and GitHub context you are operating in").
		AddReadTools(
			toolsets.NewServerTool(GetMe(getClient, t)),
		)
	contextTools.Enabled = true
	return contextTools
}

// InitDynamicToolset creates a dynamic toolset that can be used to enable other toolsets, and so requires the server and toolset group as arguments
func InitDynamicToolset(s *server.MCPServer, tsg *toolsets.ToolsetGroup, t translations.TranslationHelperFunc) *toolsets.Toolset {
	// Create a new dynamic toolset
	// Need to add the dynamic toolset last so it can be used to enable other toolsets
	dynamicToolSelection := toolsets.NewToolset("dynamic", "Discover GitHub MCP tools that can help achieve tasks by enabling additional sets of tools, you can control the enablement of any toolset to access its tools when this toolset is enabled.").
		AddReadTools(
			toolsets.NewServerTool(ListAvailableToolsets(tsg, t)),
			toolsets.NewServerTool(GetToolsetsTools(tsg, t)),
			toolsets.NewServerTool(EnableToolset(s, tsg, t)),
		)

	dynamicToolSelection.Enabled = true
	return dynamicToolSelection
}

func toBoolPtr(b bool) *bool {
	return &b
}
