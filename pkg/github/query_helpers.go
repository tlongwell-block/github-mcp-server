package github

import (
	"regexp"
	"strings"
)

// repoQueryInfo holds extracted repository information from a query
type repoQueryInfo struct {
	owner string
	repo  string
}

// extractRepoFromQuery attempts to extract repository owner and name from a search query
// It looks for patterns like "repo:owner/repo" or "owner/repo"
func extractRepoFromQuery(query string) repoQueryInfo {
	// Check for repo:owner/repo pattern
	repoPattern := regexp.MustCompile(`repo:([^/\s]+)/([^/\s]+)`)
	matches := repoPattern.FindStringSubmatch(query)
	if len(matches) == 3 {
		return repoQueryInfo{
			owner: matches[1],
			repo:  matches[2],
		}
	}

	// Check for owner/repo pattern
	ownerRepoPattern := regexp.MustCompile(`\b([^/\s]+)/([^/\s]+)\b`)
	matches = ownerRepoPattern.FindStringSubmatch(query)
	if len(matches) == 3 {
		// Make sure it's not part of another pattern
		if !strings.Contains(query, "repo:"+matches[0]) {
			return repoQueryInfo{
				owner: matches[1],
				repo:  matches[2],
			}
		}
	}

	return repoQueryInfo{}
}

// extractOrgFromQuery attempts to extract an organization or user name from a search query.
// It looks for patterns like "org:squareup" or "user:octocat".
func extractOrgFromQuery(query string) string {
	orgPattern := regexp.MustCompile(`(?:org|user):([^\s]+)`)
	matches := orgPattern.FindStringSubmatch(query)
	if len(matches) == 2 {
		return matches[1]
	}
	return ""
}
