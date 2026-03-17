package github

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_extractAllReposFromQuery(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected []repoQueryInfo
	}{
		{
			name:     "single repo: qualifier",
			query:    "repo:acme-corp/widget-bot language:go",
			expected: []repoQueryInfo{{owner: "acme-corp", repo: "widget-bot"}},
		},
		{
			name:  "multiple repo: qualifiers",
			query: "repo:acme-corp/gadget repo:globex/data-pipeline",
			expected: []repoQueryInfo{
				{owner: "acme-corp", repo: "gadget"},
				{owner: "globex", repo: "data-pipeline"},
			},
		},
		{
			name:     "no repo qualifier",
			query:    "golang test org:acme-corp",
			expected: []repoQueryInfo{},
		},
		{
			name:     "empty query",
			query:    "",
			expected: []repoQueryInfo{},
		},
		{
			name:     "mixed-case REPO: qualifier",
			query:    "REPO:acme-corp/widget-bot",
			expected: []repoQueryInfo{{owner: "acme-corp", repo: "widget-bot"}},
		},
		{
			name:     "norepo: prefix does not match",
			query:    "norepo:owner/repo language:go",
			expected: []repoQueryInfo{},
		},
		{
			name:     "repo: at start of string matches",
			query:    "repo:owner/repo language:go",
			expected: []repoQueryInfo{{owner: "owner", repo: "repo"}},
		},
		{
			name:     "repo: after space matches",
			query:    "language:go repo:owner/repo",
			expected: []repoQueryInfo{{owner: "owner", repo: "repo"}},
		},
		{
			name:     "repo: in parentheses matches",
			query:    "(repo:denied-org/secret-repo)",
			expected: []repoQueryInfo{{owner: "denied-org", repo: "secret-repo"}},
		},
		{
			name:     "repo: in double quotes matches",
			query:    `"repo:denied-org/secret-repo"`,
			expected: []repoQueryInfo{{owner: "denied-org", repo: "secret-repo"}},
		},
		{
			name:     "repo: after open paren with other qualifiers",
			query:    "(repo:org1/repo1 OR repo:org2/repo2)",
			expected: []repoQueryInfo{{owner: "org1", repo: "repo1"}, {owner: "org2", repo: "repo2"}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := extractAllReposFromQuery(tc.query)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func Test_extractAllOrgsFromQuery(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected []string
	}{
		{
			name:     "single org qualifier",
			query:    "gadget org:acme-corp",
			expected: []string{"acme-corp"},
		},
		{
			name:     "single user qualifier",
			query:    "user:octocat repos:>10",
			expected: []string{"octocat"},
		},
		{
			name:     "multiple qualifiers — catches bypass attempts",
			query:    "user:allowed-user org:denied-org",
			expected: []string{"allowed-user", "denied-org"},
		},
		{
			name:     "no org qualifier",
			query:    "golang test",
			expected: []string{},
		},
		{
			name:     "empty query",
			query:    "",
			expected: []string{},
		},
		{
			name:     "mixed-case ORG: qualifier",
			query:    "ORG:acme-corp language:go",
			expected: []string{"acme-corp"},
		},
		{
			name:     "mixed-case User: qualifier",
			query:    "User:octocat repos:>10",
			expected: []string{"octocat"},
		},
		{
			name:     "both org and repo qualifiers",
			query:    "function main repo:acme-corp/gadget org:acme-corp",
			expected: []string{"acme-corp"},
		},
		{
			name:     "org with slash-appended value extracts only org name",
			query:    "org:denied-org/subrepo language:go",
			expected: []string{"denied-org"},
		},
		{
			name:     "org: in parentheses matches",
			query:    "(org:denied-org) language:go",
			expected: []string{"denied-org"},
		},
		{
			name:     "user: in double quotes matches",
			query:    `"user:denied-user" repos:>10`,
			expected: []string{"denied-user"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := extractAllOrgsFromQuery(tc.query)
			assert.Equal(t, tc.expected, result)
		})
	}
}
