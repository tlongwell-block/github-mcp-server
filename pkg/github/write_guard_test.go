package github

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/go-github/v69/github"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/migueleliasweb/go-github-mock/src/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// dummyWriteHandler is a no-op write handler used to verify the guard passes through correctly.
func dummyWriteHandler(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultText("write succeeded"), nil
}

// dummyTool creates a minimal mcp.Tool for testing.
// Note: mcp.NewTool returns a single mcp.Tool value, NOT a tuple.
func dummyTool(name string) mcp.Tool {
	return mcp.NewTool(name, mcp.WithDescription("test tool"))
}

func Test_WritePrivateOnlyGuard(t *testing.T) {
	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectBlocked  bool
		expectedErrMsg string
		expectPassThru bool
	}{
		{
			name: "passes through when repo is private",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatch(
					mock.GetReposByOwnerByRepo,
					&github.Repository{Private: github.Ptr(true)},
				),
			),
			requestArgs: map[string]interface{}{
				"owner": "myorg",
				"repo":  "private-repo",
			},
			expectPassThru: true,
		},
		{
			name: "blocks when repo is public",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatch(
					mock.GetReposByOwnerByRepo,
					&github.Repository{Private: github.Ptr(false)},
				),
			),
			requestArgs: map[string]interface{}{
				"owner": "myorg",
				"repo":  "public-repo",
			},
			expectBlocked:  true,
			expectedErrMsg: "Write blocked: myorg/public-repo is a public repository",
		},
		{
			name: "blocks (fail-closed) when visibility check returns 404",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetReposByOwnerByRepo,
					mockResponse(t, http.StatusNotFound, `{"message": "Not Found"}`),
				),
			),
			requestArgs: map[string]interface{}{
				"owner": "myorg",
				"repo":  "any-repo",
			},
			expectBlocked:  true,
			expectedErrMsg: "Write blocked: unable to verify repository visibility",
		},
		{
			name: "blocks (fail-closed) when visibility check returns 403",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetReposByOwnerByRepo,
					mockResponse(t, http.StatusForbidden, `{"message": "Forbidden"}`),
				),
			),
			requestArgs: map[string]interface{}{
				"owner": "myorg",
				"repo":  "any-repo",
			},
			expectBlocked:  true,
			expectedErrMsg: "Write blocked: unable to verify repository visibility",
		},
		{
			name:         "returns error when owner param is missing",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]interface{}{
				"repo": "some-repo",
				// owner intentionally missing
			},
			expectBlocked:  true,
			expectedErrMsg: "missing required parameter: owner",
		},
		{
			name:         "returns error when repo param is missing",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]interface{}{
				"owner": "myorg",
				// repo intentionally missing
			},
			expectBlocked:  true,
			expectedErrMsg: "missing required parameter: repo",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := github.NewClient(tc.mockedClient)
			getClient := stubGetClientFn(client)

			tool := dummyTool("test_write_tool")
			_, guardedHandler := WritePrivateOnlyGuard(getClient, tool, dummyWriteHandler)

			request := createMCPRequest(tc.requestArgs)
			result, err := guardedHandler(context.Background(), request)

			require.NoError(t, err) // guard never returns a Go error; errors are tool results
			require.NotNil(t, result)

			textContent := getTextResult(t, result)

			if tc.expectPassThru {
				assert.Equal(t, "write succeeded", textContent.Text)
				assert.False(t, result.IsError, "expected pass-through, got error result")
			} else {
				assert.True(t, result.IsError, "expected blocked result")
				assert.Contains(t, textContent.Text, tc.expectedErrMsg)
			}
		})
	}
}

func Test_WritePrivateOnlyGuard_GetClientFailure(t *testing.T) {
	// Test that the guard fails closed when getClient returns an error
	getClient := func(_ context.Context, _ string) (*github.Client, error) {
		return nil, fmt.Errorf("auth failure: no valid credentials")
	}

	tool := dummyTool("test_write_tool")
	_, guardedHandler := WritePrivateOnlyGuard(getClient, tool, dummyWriteHandler)

	request := createMCPRequest(map[string]interface{}{
		"owner": "myorg",
		"repo":  "some-repo",
	})
	result, err := guardedHandler(context.Background(), request)
	require.NoError(t, err)
	assert.True(t, result.IsError, "expected blocked result when getClient fails")

	textContent := getTextResult(t, result)
	assert.Contains(t, textContent.Text, "unable to verify repository visibility")
}

func Test_CreateRepositoryPrivateOnlyGuard(t *testing.T) {
	tests := []struct {
		name           string
		requestArgs    map[string]interface{}
		expectBlocked  bool
		expectedErrMsg string
	}{
		{
			name: "passes through when private=true",
			requestArgs: map[string]interface{}{
				"name":    "my-repo",
				"private": true,
			},
			expectBlocked: false,
		},
		{
			name: "blocks when private=false",
			requestArgs: map[string]interface{}{
				"name":    "my-repo",
				"private": false,
			},
			expectBlocked:  true,
			expectedErrMsg: "Write blocked: create_repository requires private=true",
		},
		{
			name: "blocks when private param is absent (defaults to false)",
			requestArgs: map[string]interface{}{
				"name": "my-repo",
				// private intentionally absent
			},
			expectBlocked:  true,
			expectedErrMsg: "Write blocked: create_repository requires private=true",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tool := dummyTool("create_repository")
			_, guardedHandler := CreateRepositoryPrivateOnlyGuard(tool, dummyWriteHandler)

			request := createMCPRequest(tc.requestArgs)
			result, err := guardedHandler(context.Background(), request)

			require.NoError(t, err)
			require.NotNil(t, result)

			textContent := getTextResult(t, result)

			if tc.expectBlocked {
				assert.True(t, result.IsError)
				assert.Contains(t, textContent.Text, tc.expectedErrMsg)
			} else {
				assert.False(t, result.IsError)
				assert.Equal(t, "write succeeded", textContent.Text)
			}
		})
	}
}

func Test_ForkRepositoryPrivateOnlyGuard(t *testing.T) {
	t.Run("always blocks regardless of params", func(t *testing.T) {
		tool := dummyTool("fork_repository")
		_, guardedHandler := ForkRepositoryPrivateOnlyGuard(tool, dummyWriteHandler)

		// Try with various param combinations — should always block
		for _, args := range []map[string]interface{}{
			{"owner": "myorg", "repo": "some-repo"},
			{},
			{"owner": "myorg", "repo": "private-source"},
		} {
			request := createMCPRequest(args)
			result, err := guardedHandler(context.Background(), request)

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.True(t, result.IsError)

			textContent := getTextResult(t, result)
			assert.Contains(t, textContent.Text, "Write blocked: fork_repository cannot guarantee")
		}
	})
}

func Test_checkRepoVisibility(t *testing.T) {
	tests := []struct {
		name          string
		mockedClient  *http.Client
		owner         string
		repo          string
		expectPrivate bool
		expectError   bool
	}{
		{
			name: "returns true for private repo",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatch(
					mock.GetReposByOwnerByRepo,
					&github.Repository{Private: github.Ptr(true)},
				),
			),
			owner:         "myorg",
			repo:          "private-repo",
			expectPrivate: true,
		},
		{
			name: "returns false for public repo",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatch(
					mock.GetReposByOwnerByRepo,
					&github.Repository{Private: github.Ptr(false)},
				),
			),
			owner:         "myorg",
			repo:          "public-repo",
			expectPrivate: false,
		},
		{
			name: "returns error on API failure",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetReposByOwnerByRepo,
					mockResponse(t, http.StatusInternalServerError, `{"message": "Internal Server Error"}`),
				),
			),
			owner:       "myorg",
			repo:        "any-repo",
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := github.NewClient(tc.mockedClient)
			getClient := stubGetClientFn(client)

			isPrivate, err := checkRepoVisibility(context.Background(), getClient, tc.owner, tc.repo)

			if tc.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expectPrivate, isPrivate)
			}
		})
	}
}
