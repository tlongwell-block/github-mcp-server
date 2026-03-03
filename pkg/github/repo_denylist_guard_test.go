package github

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_RepoDenylistGuard(t *testing.T) {
	tests := []struct {
		name           string
		denylist       *RepoDenylist
		requestArgs    map[string]interface{}
		expectBlocked  bool
		expectedErrMsg string
	}{
		{
			name:        "passes through when repo is not denied",
			denylist:    NewRepoDenylist([]string{"squareup/infosec-minesweeper"}),
			requestArgs: map[string]interface{}{"owner": "squareup", "repo": "goosed-slackbot"},
		},
		{
			name:           "blocks exact match",
			denylist:       NewRepoDenylist([]string{"squareup/infosec-minesweeper"}),
			requestArgs:    map[string]interface{}{"owner": "squareup", "repo": "infosec-minesweeper"},
			expectBlocked:  true,
			expectedErrMsg: "Access blocked: squareup/infosec-minesweeper is on the repository denylist",
		},
		{
			name:           "blocks org wildcard match",
			denylist:       NewRepoDenylist([]string{"squareup/*"}),
			requestArgs:    map[string]interface{}{"owner": "squareup", "repo": "any-repo"},
			expectBlocked:  true,
			expectedErrMsg: "Access blocked: squareup/any-repo is on the repository denylist",
		},
		{
			name:           "blocks case-insensitively",
			denylist:       NewRepoDenylist([]string{"squareup/infosec-minesweeper"}),
			requestArgs:    map[string]interface{}{"owner": "SquareUp", "repo": "Infosec-Minesweeper"},
			expectBlocked:  true,
			expectedErrMsg: "Access blocked: SquareUp/Infosec-Minesweeper is on the repository denylist",
		},
		{
			name:        "passes through when denylist is empty",
			denylist:    NewRepoDenylist([]string{}),
			requestArgs: map[string]interface{}{"owner": "squareup", "repo": "infosec-minesweeper"},
		},
		{
			name:           "returns error when owner param is missing",
			denylist:       NewRepoDenylist([]string{"squareup/infosec-minesweeper"}),
			requestArgs:    map[string]interface{}{"repo": "infosec-minesweeper"},
			expectBlocked:  true,
			expectedErrMsg: "missing required parameter: owner",
		},
		{
			name:           "returns error when repo param is missing",
			denylist:       NewRepoDenylist([]string{"squareup/infosec-minesweeper"}),
			requestArgs:    map[string]interface{}{"owner": "squareup"},
			expectBlocked:  true,
			expectedErrMsg: "missing required parameter: repo",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tool := dummyTool("test_tool")
			_, guardedHandler := RepoDenylistGuard(tc.denylist, tool, dummyWriteHandler)

			result, err := guardedHandler(context.Background(), createMCPRequest(tc.requestArgs))
			require.NoError(t, err)
			require.NotNil(t, result)

			text := getTextResult(t, result)
			if tc.expectBlocked {
				assert.True(t, result.IsError)
				assert.Contains(t, text.Text, tc.expectedErrMsg)
			} else {
				assert.False(t, result.IsError, "expected pass-through, got: %s", text.Text)
				assert.Equal(t, "write succeeded", text.Text)
			}
		})
	}
}

func Test_SearchDenylistGuard(t *testing.T) {
	denylist := NewRepoDenylist([]string{"squareup/infosec-minesweeper", "afterpaytouch/*"})

	tests := []struct {
		name           string
		queryParam     string
		query          string
		expectBlocked  bool
		expectedErrMsg string
	}{
		{
			name:       "passes through unrelated query",
			queryParam: "q",
			query:      "some search term",
		},
		{
			name:           "blocks repo: qualifier targeting denied repo",
			queryParam:     "q",
			query:          "repo:squareup/infosec-minesweeper secret",
			expectBlocked:  true,
			expectedErrMsg: "Search blocked: the query targets squareup/infosec-minesweeper",
		},
		{
			name:           "blocks org: qualifier targeting denied org",
			queryParam:     "q",
			query:          "org:afterpaytouch authentication",
			expectBlocked:  true,
			expectedErrMsg: "Search blocked: the query targets organization afterpaytouch",
		},
		{
			name:       "passes through org: qualifier for non-denied org",
			queryParam: "q",
			query:      "org:squareup some-term",
		},
		{
			name:       "passes through repo: qualifier targeting non-denied repo",
			queryParam: "q",
			query:      "repo:squareup/goosed-slackbot something",
		},
		{
			name:       "passes through query with no qualifiers",
			queryParam: "query",
			query:      "repositories about authentication",
		},
		{
			name:           "blocks second repo: qualifier when first is allowed (multi-qualifier bypass)",
			queryParam:     "q",
			query:          "repo:squareup/goosed-slackbot repo:squareup/infosec-minesweeper something",
			expectBlocked:  true,
			expectedErrMsg: "Search blocked: the query targets squareup/infosec-minesweeper",
		},
		{
			name:           "blocks user: qualifier targeting denied org",
			queryParam:     "q",
			query:          "user:afterpaytouch something",
			expectBlocked:  true,
			expectedErrMsg: "Search blocked: the query targets organization afterpaytouch",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tool := dummyTool("search_tool")
			args := map[string]interface{}{tc.queryParam: tc.query}
			_, guardedHandler := SearchDenylistGuard(denylist, tc.queryParam, tool, dummyWriteHandler)

			result, err := guardedHandler(context.Background(), createMCPRequest(args))
			require.NoError(t, err)
			require.NotNil(t, result)

			text := getTextResult(t, result)
			if tc.expectBlocked {
				assert.True(t, result.IsError)
				assert.Contains(t, text.Text, tc.expectedErrMsg)
			} else {
				assert.False(t, result.IsError, "expected pass-through, got: %s", text.Text)
				assert.Equal(t, "write succeeded", text.Text)
			}
		})
	}
}

func Test_DenylistResourceGuard(t *testing.T) {
	dummyResourceHandler := func(_ context.Context, _ mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		return []mcp.ResourceContents{mcp.TextResourceContents{URI: "repo://ok", Text: "content"}}, nil
	}

	tests := []struct {
		name          string
		denylist      *RepoDenylist
		owner         string
		repo          string
		expectBlocked bool
	}{
		{
			name:     "passes through when repo is not denied",
			denylist: NewRepoDenylist([]string{"squareup/infosec-minesweeper"}),
			owner:    "squareup",
			repo:     "goosed-slackbot",
		},
		{
			name:          "blocks denied repo",
			denylist:      NewRepoDenylist([]string{"squareup/infosec-minesweeper"}),
			owner:         "squareup",
			repo:          "infosec-minesweeper",
			expectBlocked: true,
		},
		{
			name:          "blocks org wildcard",
			denylist:      NewRepoDenylist([]string{"squareup/*"}),
			owner:         "squareup",
			repo:          "any-repo",
			expectBlocked: true,
		},
		{
			name:     "passes through when denylist is empty",
			denylist: NewRepoDenylist(nil),
			owner:    "squareup",
			repo:     "infosec-minesweeper",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			guarded := DenylistResourceGuard(tc.denylist, dummyResourceHandler)

			req := mcp.ReadResourceRequest{}
			req.Params.Arguments = map[string]interface{}{
				"owner": []string{tc.owner},
				"repo":  []string{tc.repo},
			}

			contents, err := guarded(context.Background(), req)
			if tc.expectBlocked {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "Access blocked")
				assert.Nil(t, contents)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, contents)
			}
		})
	}
}
