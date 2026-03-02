package github

import (
	"context"
	"testing"

	"github.com/github/github-mcp-server/pkg/toolsets"
	"github.com/google/go-github/v69/github"
	"github.com/shurcooL/githubv4"
)

func TestInitToolsetsWithConfig(t *testing.T) {
	// Mock translation function
	mockTranslator := func(key, fallback string) string {
		return fallback
	}

	tests := []struct {
		name     string
		configs  []toolsets.ToolsetConfig
		readOnly bool
		wantErr  bool
		expected map[string]toolsets.ToolsetMode
	}{
		{
			name: "enable repos with rw and issues with ro",
			configs: []toolsets.ToolsetConfig{
				{Name: "repos", Mode: toolsets.ReadWrite},
				{Name: "issues", Mode: toolsets.ReadOnly},
			},
			readOnly: false,
			wantErr:  false,
			expected: map[string]toolsets.ToolsetMode{
				"repos":  toolsets.ReadWrite,
				"issues": toolsets.ReadOnly,
			},
		},
		{
			name: "enable all with readonly mode",
			configs: []toolsets.ToolsetConfig{
				{Name: "all", Mode: toolsets.ReadOnly},
			},
			readOnly: false,
			wantErr:  false,
			expected: map[string]toolsets.ToolsetMode{
				"repos":             toolsets.ReadOnly,
				"issues":            toolsets.ReadOnly,
				"users":             toolsets.ReadOnly,
				"pull_requests":     toolsets.ReadOnly,
				"code_security":     toolsets.ReadOnly,
				"secret_protection": toolsets.ReadOnly,
				"experiments":       toolsets.ReadOnly,
				"context":           toolsets.ReadOnly,
			},
		},
		{
			name: "enable nonexistent toolset",
			configs: []toolsets.ToolsetConfig{
				{Name: "nonexistent", Mode: toolsets.ReadWrite},
			},
			readOnly: false,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock client functions
			getClient := func(ctx context.Context, _ string) (*github.Client, error) {
				return nil, nil
			}
			getGQLClient := func(ctx context.Context, _ string) (*githubv4.Client, error) {
				return nil, nil
			}

			tsg, err := InitToolsetsWithConfig(tt.configs, tt.readOnly, false, getClient, getGQLClient, mockTranslator)

			if tt.wantErr {
				if err == nil {
					t.Errorf("InitToolsetsWithConfig() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("InitToolsetsWithConfig() unexpected error: %v", err)
				return
			}

			// Check that expected toolsets are enabled with correct modes
			for name, expectedMode := range tt.expected {
				toolset, exists := tsg.Toolsets[name]
				if !exists {
					t.Errorf("Expected toolset %s to exist", name)
					continue
				}
				if !toolset.Enabled {
					t.Errorf("Expected toolset %s to be enabled", name)
				}
				if toolset.Mode != expectedMode {
					t.Errorf("Expected toolset %s to have mode %s, got %s", name, expectedMode, toolset.Mode)
				}
			}

			// Check that non-expected toolsets are not enabled
			for name, toolset := range tsg.Toolsets {
				if _, expected := tt.expected[name]; !expected && toolset.Enabled {
					t.Errorf("Expected toolset %s to not be enabled", name)
				}
			}
		})
	}
}

func TestInitToolsets_BackwardCompatibility(t *testing.T) {
	// Mock translation function
	mockTranslator := func(key, fallback string) string {
		return fallback
	}

	// Mock client functions
	getClient := func(ctx context.Context, _ string) (*github.Client, error) {
		return nil, nil
	}
	getGQLClient := func(ctx context.Context, _ string) (*githubv4.Client, error) {
		return nil, nil
	}

	tests := []struct {
		name            string
		passedToolsets  []string
		readOnly        bool
		wantErr         bool
		expectedEnabled []string
	}{
		{
			name:            "legacy format - single toolset",
			passedToolsets:  []string{"repos"},
			readOnly:        false,
			wantErr:         false,
			expectedEnabled: []string{"repos"},
		},
		{
			name:            "legacy format - multiple toolsets",
			passedToolsets:  []string{"repos", "issues", "users"},
			readOnly:        false,
			wantErr:         false,
			expectedEnabled: []string{"repos", "issues", "users"},
		},
		{
			name:            "legacy format - all",
			passedToolsets:  []string{"all"},
			readOnly:        false,
			wantErr:         false,
			expectedEnabled: []string{"repos", "issues", "users", "pull_requests", "code_security", "secret_protection", "experiments", "context"},
		},
		{
			name:            "new format - mixed modes",
			passedToolsets:  []string{"repos:rw,issues:ro,users"},
			readOnly:        false,
			wantErr:         false,
			expectedEnabled: []string{"repos", "issues", "users"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tsg, err := InitToolsets(tt.passedToolsets, tt.readOnly, false, getClient, getGQLClient, mockTranslator)

			if tt.wantErr {
				if err == nil {
					t.Errorf("InitToolsets() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("InitToolsets() unexpected error: %v", err)
				return
			}

			// Check that expected toolsets are enabled
			for _, name := range tt.expectedEnabled {
				toolset, exists := tsg.Toolsets[name]
				if !exists {
					t.Errorf("Expected toolset %s to exist", name)
					continue
				}
				if !toolset.Enabled {
					t.Errorf("Expected toolset %s to be enabled", name)
				}
			}
		})
	}
}

func TestToolsetModeFiltering(t *testing.T) {
	// Mock translation function
	mockTranslator := func(key, fallback string) string {
		return fallback
	}

	// Mock client functions
	getClient := func(ctx context.Context, _ string) (*github.Client, error) {
		return nil, nil
	}
	getGQLClient := func(ctx context.Context, _ string) (*githubv4.Client, error) {
		return nil, nil
	}

	tests := []struct {
		name             string
		configs          []toolsets.ToolsetConfig
		toolsetName      string
		expectWriteTools bool
		expectReadTools  bool
	}{
		{
			name: "repos toolset in ReadWrite mode should have both read and write tools",
			configs: []toolsets.ToolsetConfig{
				{Name: "repos", Mode: toolsets.ReadWrite},
			},
			toolsetName:      "repos",
			expectWriteTools: true,
			expectReadTools:  true,
		},
		{
			name: "repos toolset in ReadOnly mode should have only read tools",
			configs: []toolsets.ToolsetConfig{
				{Name: "repos", Mode: toolsets.ReadOnly},
			},
			toolsetName:      "repos",
			expectWriteTools: false,
			expectReadTools:  true,
		},
		{
			name: "pull_requests toolset in ReadOnly mode should have only read tools",
			configs: []toolsets.ToolsetConfig{
				{Name: "pull_requests", Mode: toolsets.ReadOnly},
			},
			toolsetName:      "pull_requests",
			expectWriteTools: false,
			expectReadTools:  true,
		},
		{
			name: "pull_requests toolset in ReadWrite mode should have both read and write tools",
			configs: []toolsets.ToolsetConfig{
				{Name: "pull_requests", Mode: toolsets.ReadWrite},
			},
			toolsetName:      "pull_requests",
			expectWriteTools: true,
			expectReadTools:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tsg, err := InitToolsetsWithConfig(tt.configs, false, false, getClient, getGQLClient, mockTranslator)
			if err != nil {
				t.Fatalf("InitToolsetsWithConfig() error: %v", err)
			}

			toolset, exists := tsg.Toolsets[tt.toolsetName]
			if !exists {
				t.Fatalf("Expected toolset %s to exist", tt.toolsetName)
			}

			if !toolset.Enabled {
				t.Fatalf("Expected toolset %s to be enabled", tt.toolsetName)
			}

			activeTools := toolset.GetActiveTools()
			availableTools := toolset.GetAvailableTools()

			// Count read and write tools
			readToolCount := 0
			writeToolCount := 0

			for _, tool := range activeTools {
				if tool.Tool.Annotations.ReadOnlyHint != nil && *tool.Tool.Annotations.ReadOnlyHint {
					readToolCount++
				} else {
					writeToolCount++
				}
			}

			// Verify expectations
			if tt.expectReadTools && readToolCount == 0 {
				t.Errorf("Expected toolset %s to have read tools, but found none", tt.toolsetName)
			}

			if !tt.expectReadTools && readToolCount > 0 {
				t.Errorf("Expected toolset %s to have no read tools, but found %d", tt.toolsetName, readToolCount)
			}

			if tt.expectWriteTools && writeToolCount == 0 {
				t.Errorf("Expected toolset %s to have write tools, but found none", tt.toolsetName)
			}

			if !tt.expectWriteTools && writeToolCount > 0 {
				t.Errorf("Expected toolset %s to have no write tools, but found %d", tt.toolsetName, writeToolCount)
			}

			// Verify that GetActiveTools and GetAvailableTools behave correctly
			if tt.expectWriteTools {
				if len(activeTools) != len(availableTools) {
					t.Errorf("For ReadWrite mode, active tools (%d) should equal available tools (%d)", len(activeTools), len(availableTools))
				}
			} else {
				// In ReadOnly mode, active tools should be a subset of available tools
				if len(activeTools) > len(availableTools) {
					t.Errorf("Active tools (%d) should not exceed available tools (%d)", len(activeTools), len(availableTools))
				}
			}

			t.Logf("Toolset %s (%s mode): %d read tools, %d write tools, %d active tools, %d available tools",
				tt.toolsetName, toolset.Mode, readToolCount, writeToolCount, len(activeTools), len(availableTools))
		})
	}
}

func TestContextToolsetIntegration(t *testing.T) {
	// Mock translation function
	mockTranslator := func(key, fallback string) string {
		return fallback
	}

	// Mock client functions
	getClient := func(ctx context.Context, _ string) (*github.Client, error) {
		return nil, nil
	}
	getGQLClient := func(ctx context.Context, _ string) (*githubv4.Client, error) {
		return nil, nil
	}

	// Test that context toolset can be configured
	configs := []toolsets.ToolsetConfig{
		{Name: "context", Mode: toolsets.ReadWrite},
		{Name: "repos", Mode: toolsets.ReadOnly},
	}

	tsg, err := InitToolsetsWithConfig(configs, false, false, getClient, getGQLClient, mockTranslator)
	if err != nil {
		t.Fatalf("InitToolsetsWithConfig() error: %v", err)
	}

	// Verify context toolset exists and is enabled
	contextToolset, exists := tsg.Toolsets["context"]
	if !exists {
		t.Fatalf("Expected context toolset to exist")
	}

	if !contextToolset.Enabled {
		t.Errorf("Expected context toolset to be enabled")
	}

	if contextToolset.Mode != toolsets.ReadWrite {
		t.Errorf("Expected context toolset to have ReadWrite mode, got %s", contextToolset.Mode)
	}

	// Verify context toolset has the expected tools
	activeTools := contextToolset.GetActiveTools()
	if len(activeTools) == 0 {
		t.Errorf("Expected context toolset to have tools")
	}

	// Check that we have the get_me tool
	found := false
	for _, tool := range activeTools {
		if tool.Tool.Name == "get_me" {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Expected context toolset to have get_me tool")
	}

	t.Logf("Context toolset has %d active tools", len(activeTools))
}

func TestWritePrivateOnlyGuardWiring(t *testing.T) {
	// Verify that when writePrivateOnly=true, write tools are wrapped with guards
	// and that when writePrivateOnly=false, they are not.
	mockTranslator := func(key, fallback string) string {
		return fallback
	}
	getClient := func(ctx context.Context, _ string) (*github.Client, error) {
		return github.NewClient(nil), nil
	}
	getGQLClient := func(ctx context.Context, _ string) (*githubv4.Client, error) {
		return nil, nil
	}

	configs := []toolsets.ToolsetConfig{
		{Name: "all", Mode: toolsets.ReadWrite},
	}

	// Initialize with writePrivateOnly=false
	tsgOff, err := InitToolsetsWithConfig(configs, false, false, getClient, getGQLClient, mockTranslator)
	if err != nil {
		t.Fatalf("InitToolsetsWithConfig(writePrivateOnly=false) error: %v", err)
	}

	// Initialize with writePrivateOnly=true
	tsgOn, err := InitToolsetsWithConfig(configs, false, true, getClient, getGQLClient, mockTranslator)
	if err != nil {
		t.Fatalf("InitToolsetsWithConfig(writePrivateOnly=true) error: %v", err)
	}

	// Both should have the same number of toolsets
	if len(tsgOff.Toolsets) != len(tsgOn.Toolsets) {
		t.Errorf("Expected same number of toolsets, got %d vs %d", len(tsgOff.Toolsets), len(tsgOn.Toolsets))
	}

	// Both should have the same tools (write tools are still registered, just wrapped)
	for name, tsOff := range tsgOff.Toolsets {
		tsOn, exists := tsgOn.Toolsets[name]
		if !exists {
			t.Errorf("Toolset %s missing from writePrivateOnly=true", name)
			continue
		}
		offTools := tsOff.GetActiveTools()
		onTools := tsOn.GetActiveTools()
		if len(offTools) != len(onTools) {
			t.Errorf("Toolset %s: expected %d tools, got %d with writePrivateOnly=true",
				name, len(offTools), len(onTools))
		}
	}

	// Verify that fork_repository is blocked when writePrivateOnly=true
	// by calling the handler directly
	repoToolset := tsgOn.Toolsets["repos"]
	if repoToolset == nil {
		t.Fatal("repos toolset not found")
	}
	for _, tool := range repoToolset.GetActiveTools() {
		if tool.Tool.Name == "fork_repository" {
			result, err := tool.Handler(context.Background(), createMCPRequest(map[string]interface{}{
				"owner": "testowner",
				"repo":  "testrepo",
			}))
			if err != nil {
				t.Fatalf("fork_repository handler returned error: %v", err)
			}
			// Should be blocked
			if result == nil || !result.IsError {
				t.Error("Expected fork_repository to be blocked when writePrivateOnly=true")
			}
			return
		}
	}
	t.Error("fork_repository tool not found in repos toolset")
}
