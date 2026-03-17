package github

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRepoDenylist_IsDenied(t *testing.T) {
	tests := []struct {
		name     string
		entries  []string
		owner    string
		repo     string
		expected bool
	}{
		{
			name:     "exact match is denied",
			entries:  []string{"acme-corp/internal-dashboard"},
			owner:    "acme-corp",
			repo:     "internal-dashboard",
			expected: true,
		},
		{
			name:     "non-denied repo is allowed",
			entries:  []string{"acme-corp/internal-dashboard"},
			owner:    "acme-corp",
			repo:     "widget-bot",
			expected: false,
		},
		{
			name:     "org wildcard denies any repo under org",
			entries:  []string{"acme-corp/*"},
			owner:    "acme-corp",
			repo:     "any-repo",
			expected: true,
		},
		{
			name:     "org wildcard does not affect other orgs",
			entries:  []string{"acme-corp/*"},
			owner:    "otherorg",
			repo:     "any-repo",
			expected: false,
		},
		{
			name:     "case-insensitive owner match",
			entries:  []string{"Acme-Corp/internal-dashboard"},
			owner:    "acme-corp",
			repo:     "internal-dashboard",
			expected: true,
		},
		{
			name:     "case-insensitive repo match",
			entries:  []string{"acme-corp/Internal-Dashboard"},
			owner:    "acme-corp",
			repo:     "internal-dashboard",
			expected: true,
		},
		{
			name:     "case-insensitive input",
			entries:  []string{"acme-corp/internal-dashboard"},
			owner:    "Acme-Corp",
			repo:     "Internal-Dashboard",
			expected: true,
		},
		{
			name:     "empty denylist allows everything",
			entries:  []string{},
			owner:    "acme-corp",
			repo:     "any-repo",
			expected: false,
		},
		{
			name:     "whitespace around entries is trimmed",
			entries:  []string{"  acme-corp/internal-dashboard  "},
			owner:    "acme-corp",
			repo:     "internal-dashboard",
			expected: true,
		},
		{
			name:     "comma-separated entries parsed correctly (multiple)",
			entries:  []string{"acme-corp/internal-dashboard", "acme-corp/compliance-tracker"},
			owner:    "acme-corp",
			repo:     "compliance-tracker",
			expected: true,
		},
		{
			name:     "invalid entry without slash is skipped",
			entries:  []string{"noslash", "acme-corp/valid-repo"},
			owner:    "acme-corp",
			repo:     "valid-repo",
			expected: true,
		},
		{
			name:     "empty string entries are skipped",
			entries:  []string{"", "acme-corp/internal-dashboard"},
			owner:    "acme-corp",
			repo:     "internal-dashboard",
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := NewRepoDenylist(tc.entries)
			assert.Equal(t, tc.expected, d.IsDenied(tc.owner, tc.repo))
		})
	}
}

func TestRepoDenylist_NilSafe(t *testing.T) {
	var d *RepoDenylist
	assert.False(t, d.IsDenied("acme-corp", "any-repo"))
	assert.False(t, d.IsOrgDenied("acme-corp"))
	assert.True(t, d.IsEmpty())
}

func TestRepoDenylist_IsEmpty(t *testing.T) {
	assert.True(t, NewRepoDenylist(nil).IsEmpty())
	assert.True(t, NewRepoDenylist([]string{}).IsEmpty())
	assert.True(t, NewRepoDenylist([]string{"", "  "}).IsEmpty())
	assert.False(t, NewRepoDenylist([]string{"acme-corp/repo"}).IsEmpty())
	assert.False(t, NewRepoDenylist([]string{"acme-corp/*"}).IsEmpty())
}

func TestRepoDenylist_IsOrgDenied(t *testing.T) {
	d := NewRepoDenylist([]string{"acme-corp/*", "partner-org/specific-repo"})

	// Org wildcard entry
	assert.True(t, d.IsOrgDenied("acme-corp"))
	assert.True(t, d.IsOrgDenied("Acme-Corp")) // case-insensitive

	// Exact entry does not make the whole org denied
	assert.False(t, d.IsOrgDenied("partner-org"))

	// Unrelated org
	assert.False(t, d.IsOrgDenied("someotherorg"))
}
