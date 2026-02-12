package toolsets

import (
	"testing"
)

func TestParseToolsetConfig(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []ToolsetConfig
		wantErr  bool
	}{
		{
			name:     "empty string",
			input:    "",
			expected: []ToolsetConfig{},
			wantErr:  false,
		},
		{
			name:  "single toolset without mode",
			input: "repos",
			expected: []ToolsetConfig{
				{Name: "repos", Mode: ReadWrite},
			},
			wantErr: false,
		},
		{
			name:  "single toolset with rw mode",
			input: "repos:rw",
			expected: []ToolsetConfig{
				{Name: "repos", Mode: ReadWrite},
			},
			wantErr: false,
		},
		{
			name:  "single toolset with ro mode",
			input: "repos:ro",
			expected: []ToolsetConfig{
				{Name: "repos", Mode: ReadOnly},
			},
			wantErr: false,
		},
		{
			name:  "single toolset with readwrite mode",
			input: "repos:readwrite",
			expected: []ToolsetConfig{
				{Name: "repos", Mode: ReadWrite},
			},
			wantErr: false,
		},
		{
			name:  "single toolset with readonly mode",
			input: "repos:readonly",
			expected: []ToolsetConfig{
				{Name: "repos", Mode: ReadOnly},
			},
			wantErr: false,
		},
		{
			name:  "multiple toolsets mixed modes",
			input: "repos:rw,issues:ro,users",
			expected: []ToolsetConfig{
				{Name: "repos", Mode: ReadWrite},
				{Name: "issues", Mode: ReadOnly},
				{Name: "users", Mode: ReadWrite},
			},
			wantErr: false,
		},
		{
			name:  "multiple toolsets with spaces",
			input: "repos:rw, issues:ro, users",
			expected: []ToolsetConfig{
				{Name: "repos", Mode: ReadWrite},
				{Name: "issues", Mode: ReadOnly},
				{Name: "users", Mode: ReadWrite},
			},
			wantErr: false,
		},
		{
			name:  "all toolset with mode",
			input: "all:ro",
			expected: []ToolsetConfig{
				{Name: "all", Mode: ReadOnly},
			},
			wantErr: false,
		},
		{
			name:    "invalid mode",
			input:   "repos:invalid",
			wantErr: true,
		},
		{
			name:    "empty toolset name",
			input:   ":rw",
			wantErr: true,
		},
		{
			name:    "malformed format",
			input:   "repos:rw:extra",
			wantErr: true,
		},
		{
			name:  "case insensitive modes",
			input: "repos:RW,issues:RO",
			expected: []ToolsetConfig{
				{Name: "repos", Mode: ReadWrite},
				{Name: "issues", Mode: ReadOnly},
			},
			wantErr: false,
		},
		{
			name:  "empty items in list",
			input: "repos,,issues:ro,",
			expected: []ToolsetConfig{
				{Name: "repos", Mode: ReadWrite},
				{Name: "issues", Mode: ReadOnly},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseToolsetConfig(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseToolsetConfig() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("ParseToolsetConfig() unexpected error: %v", err)
				return
			}

			if len(result) != len(tt.expected) {
				t.Errorf("ParseToolsetConfig() got %d configs, expected %d", len(result), len(tt.expected))
				return
			}

			for i, config := range result {
				if config.Name != tt.expected[i].Name {
					t.Errorf("ParseToolsetConfig() config[%d].Name = %s, expected %s", i, config.Name, tt.expected[i].Name)
				}
				if config.Mode != tt.expected[i].Mode {
					t.Errorf("ParseToolsetConfig() config[%d].Mode = %s, expected %s", i, config.Mode, tt.expected[i].Mode)
				}
			}
		})
	}
}

func TestParseToolsetConfigFromSlice(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []ToolsetConfig
		wantErr  bool
	}{
		{
			name:     "empty slice",
			input:    []string{},
			expected: []ToolsetConfig{},
			wantErr:  false,
		},
		{
			name:  "single item",
			input: []string{"repos:rw"},
			expected: []ToolsetConfig{
				{Name: "repos", Mode: ReadWrite},
			},
			wantErr: false,
		},
		{
			name:  "multiple items",
			input: []string{"repos:rw", "issues:ro"},
			expected: []ToolsetConfig{
				{Name: "repos", Mode: ReadWrite},
				{Name: "issues", Mode: ReadOnly},
			},
			wantErr: false,
		},
		{
			name:  "comma-separated in single item",
			input: []string{"repos:rw,issues:ro,users"},
			expected: []ToolsetConfig{
				{Name: "repos", Mode: ReadWrite},
				{Name: "issues", Mode: ReadOnly},
				{Name: "users", Mode: ReadWrite},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseToolsetConfigFromSlice(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseToolsetConfigFromSlice() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("ParseToolsetConfigFromSlice() unexpected error: %v", err)
				return
			}

			if len(result) != len(tt.expected) {
				t.Errorf("ParseToolsetConfigFromSlice() got %d configs, expected %d", len(result), len(tt.expected))
				return
			}

			for i, config := range result {
				if config.Name != tt.expected[i].Name {
					t.Errorf("ParseToolsetConfigFromSlice() config[%d].Name = %s, expected %s", i, config.Name, tt.expected[i].Name)
				}
				if config.Mode != tt.expected[i].Mode {
					t.Errorf("ParseToolsetConfigFromSlice() config[%d].Mode = %s, expected %s", i, config.Mode, tt.expected[i].Mode)
				}
			}
		})
	}
}

func TestToolsetModeString(t *testing.T) {
	tests := []struct {
		mode     ToolsetMode
		expected string
	}{
		{ReadWrite, "rw"},
		{ReadOnly, "ro"},
		{ToolsetMode(999), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.mode.String()
			if result != tt.expected {
				t.Errorf("ToolsetMode.String() = %s, expected %s", result, tt.expected)
			}
		})
	}
}
