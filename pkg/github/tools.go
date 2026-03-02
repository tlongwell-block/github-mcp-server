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

func InitToolsets(passedToolsets []string, readOnly bool, writePrivateOnly bool, getClient GetClientFn, getGQLClient GetGQLClientFn, t translations.TranslationHelperFunc) (*toolsets.ToolsetGroup, error) {
	// Parse toolset configurations from the passed toolsets
	configs, err := toolsets.ParseToolsetConfigFromSlice(passedToolsets)
	if err != nil {
		return nil, fmt.Errorf("failed to parse toolset configuration: %w", err)
	}

	return InitToolsetsWithConfig(configs, readOnly, writePrivateOnly, getClient, getGQLClient, t)
}

func InitToolsetsWithConfig(configs []toolsets.ToolsetConfig, readOnly bool, writePrivateOnly bool, getClient GetClientFn, getGQLClient GetGQLClientFn, t translations.TranslationHelperFunc) (*toolsets.ToolsetGroup, error) {
	// Create a new toolset group
	tsg := toolsets.NewToolsetGroup(readOnly)

	// Helper functions to conditionally wrap write tool handlers with guards.
	// When writePrivateOnly=false, these are no-ops that return the tool and handler unchanged.
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
			toolsets.NewServerTool(SearchRepositories(getClient, t)),
			toolsets.NewServerTool(GetFileContents(getClient, t)),
			toolsets.NewServerTool(ListCommits(getClient, t)),
			toolsets.NewServerTool(SearchCode(getClient, t)),
			toolsets.NewServerTool(GetCommit(getClient, t)),
			toolsets.NewServerTool(ListBranches(getClient, t)),
			toolsets.NewServerTool(ListTags(getClient, t)),
			toolsets.NewServerTool(GetTag(getClient, t)),
		).
		AddWriteTools(
			toolsets.NewServerTool(guardWrite(CreateOrUpdateFile(getClient, t))),
			toolsets.NewServerTool(guardCreate(CreateRepository(getClient, t))),
			toolsets.NewServerTool(guardFork(ForkRepository(getClient, t))),
			toolsets.NewServerTool(guardWrite(CreateBranch(getClient, t))),
			toolsets.NewServerTool(guardWrite(PushFiles(getClient, t))),
			toolsets.NewServerTool(guardWrite(DeleteFile(getClient, t))),
		)
	issues := toolsets.NewToolset("issues", "GitHub Issues related tools").
		AddReadTools(
			toolsets.NewServerTool(GetIssue(getClient, t)),
			toolsets.NewServerTool(SearchIssues(getClient, t)),
			toolsets.NewServerTool(ListIssues(getClient, t)),
			toolsets.NewServerTool(GetIssueComments(getClient, t)),
		).
		AddWriteTools(
			toolsets.NewServerTool(guardWrite(CreateIssue(getClient, t))),
			toolsets.NewServerTool(guardWrite(AddIssueComment(getClient, t))),
			toolsets.NewServerTool(guardWrite(UpdateIssue(getClient, t))),
		)
	users := toolsets.NewToolset("users", "GitHub User related tools").
		AddReadTools(
			toolsets.NewServerTool(SearchUsers(getClient, t)),
		)
	pullRequests := toolsets.NewToolset("pull_requests", "GitHub Pull Request related tools").
		AddReadTools(
			toolsets.NewServerTool(GetPullRequest(getClient, t)),
			toolsets.NewServerTool(ListPullRequests(getClient, t)),
			toolsets.NewServerTool(GetPullRequestFiles(getClient, t)),
			toolsets.NewServerTool(GetPullRequestStatus(getClient, t)),
			toolsets.NewServerTool(GetPullRequestComments(getClient, t)),
			toolsets.NewServerTool(GetPullRequestReviews(getClient, t)),
			toolsets.NewServerTool(GetPullRequestDiff(getClient, t)),
		).
		AddWriteTools(
			toolsets.NewServerTool(guardWrite(MergePullRequest(getClient, t))),
			toolsets.NewServerTool(guardWrite(UpdatePullRequestBranch(getClient, t))),
			toolsets.NewServerTool(guardWrite(CreatePullRequest(getClient, t))),
			toolsets.NewServerTool(guardWrite(UpdatePullRequest(getClient, t))),
			toolsets.NewServerTool(guardWrite(RequestCopilotReview(getClient, t))),

			// Reviews
			toolsets.NewServerTool(guardWrite(CreateAndSubmitPullRequestReview(getGQLClient, t))),
			toolsets.NewServerTool(guardWrite(CreatePendingPullRequestReview(getGQLClient, t))),
			toolsets.NewServerTool(guardWrite(AddPullRequestReviewCommentToPendingReview(getGQLClient, t))),
			toolsets.NewServerTool(guardWrite(SubmitPendingPullRequestReview(getGQLClient, t))),
			toolsets.NewServerTool(guardWrite(DeletePendingPullRequestReview(getGQLClient, t))),
		)
	codeSecurity := toolsets.NewToolset("code_security", "Code security related tools, such as GitHub Code Scanning").
		AddReadTools(
			toolsets.NewServerTool(GetCodeScanningAlert(getClient, t)),
			toolsets.NewServerTool(ListCodeScanningAlerts(getClient, t)),
		)
	secretProtection := toolsets.NewToolset("secret_protection", "Secret protection related tools, such as GitHub Secret Scanning").
		AddReadTools(
			toolsets.NewServerTool(GetSecretScanningAlert(getClient, t)),
			toolsets.NewServerTool(ListSecretScanningAlerts(getClient, t)),
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
