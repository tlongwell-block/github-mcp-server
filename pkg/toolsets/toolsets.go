package toolsets

import (
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func NewServerTool(tool mcp.Tool, handler server.ToolHandlerFunc) server.ServerTool {
	return server.ServerTool{Tool: tool, Handler: handler}
}

type Toolset struct {
	Name        string
	Description string
	Enabled     bool
	Mode        ToolsetMode // NEW: per-toolset mode
	readOnly    bool        // Global read-only override (deprecated in favor of Mode)
	writeTools  []server.ServerTool
	readTools   []server.ServerTool
}

func (t *Toolset) GetActiveTools() []server.ServerTool {
	if t.Enabled {
		// Check both global readOnly and per-toolset Mode
		if t.readOnly || t.Mode == ReadOnly {
			return t.readTools
		}
		return append(t.readTools, t.writeTools...)
	}
	return nil
}

func (t *Toolset) GetAvailableTools() []server.ServerTool {
	// Check both global readOnly and per-toolset Mode
	if t.readOnly || t.Mode == ReadOnly {
		return t.readTools
	}
	return append(t.readTools, t.writeTools...)
}

func (t *Toolset) RegisterTools(s *server.MCPServer) {
	if !t.Enabled {
		return
	}
	for _, tool := range t.readTools {
		s.AddTool(tool.Tool, tool.Handler)
	}
	// Only register write tools if both global and per-toolset settings allow it
	if !t.readOnly && t.Mode != ReadOnly {
		for _, tool := range t.writeTools {
			s.AddTool(tool.Tool, tool.Handler)
		}
	}
}

func (t *Toolset) SetReadOnly() {
	// Set the toolset to read-only
	t.readOnly = true
}

func (t *Toolset) AddWriteTools(tools ...server.ServerTool) *Toolset {
	// Silently ignore if the toolset is read-only to avoid any breach of that contract
	for _, tool := range tools {
		if *tool.Tool.Annotations.ReadOnlyHint {
			panic(fmt.Sprintf("tool (%s) is incorrectly annotated as read-only", tool.Tool.Name))
		}
	}
	if !t.readOnly {
		t.writeTools = append(t.writeTools, tools...)
	}
	return t
}

func (t *Toolset) AddReadTools(tools ...server.ServerTool) *Toolset {
	for _, tool := range tools {
		if !*tool.Tool.Annotations.ReadOnlyHint {
			panic(fmt.Sprintf("tool (%s) must be annotated as read-only", tool.Tool.Name))
		}
	}
	t.readTools = append(t.readTools, tools...)
	return t
}

type ToolsetGroup struct {
	Toolsets     map[string]*Toolset
	everythingOn bool
	readOnly     bool
}

func NewToolsetGroup(readOnly bool) *ToolsetGroup {
	return &ToolsetGroup{
		Toolsets:     make(map[string]*Toolset),
		everythingOn: false,
		readOnly:     readOnly,
	}
}

func (tg *ToolsetGroup) AddToolset(ts *Toolset) {
	if tg.readOnly {
		ts.SetReadOnly()
	}
	tg.Toolsets[ts.Name] = ts
}

func NewToolset(name string, description string) *Toolset {
	return &Toolset{
		Name:        name,
		Description: description,
		Enabled:     false,
		Mode:        ReadWrite, // Default to ReadWrite
		readOnly:    false,
	}
}

func (tg *ToolsetGroup) IsEnabled(name string) bool {
	// If everythingOn is true, all features are enabled
	if tg.everythingOn {
		return true
	}

	feature, exists := tg.Toolsets[name]
	if !exists {
		return false
	}
	return feature.Enabled
}

func (tg *ToolsetGroup) EnableToolsetsWithConfig(configs []ToolsetConfig) error {
	// Special case for "all"
	for _, config := range configs {
		if config.Name == "all" {
			tg.everythingOn = true
			// Apply the mode to all toolsets
			for name := range tg.Toolsets {
				toolset := tg.Toolsets[name]
				toolset.Enabled = true
				toolset.Mode = config.Mode
				tg.Toolsets[name] = toolset
			}
			return nil
		}
	}

	// Enable specific toolsets with their modes
	for _, config := range configs {
		toolset, exists := tg.Toolsets[config.Name]
		if !exists {
			return fmt.Errorf("toolset %s does not exist", config.Name)
		}
		toolset.Enabled = true
		toolset.Mode = config.Mode
		tg.Toolsets[config.Name] = toolset
	}

	return nil
}

func (tg *ToolsetGroup) EnableToolsets(names []string) error {
	// Special case for "all"
	for _, name := range names {
		if name == "all" {
			tg.everythingOn = true
			break
		}
		err := tg.EnableToolset(name)
		if err != nil {
			return err
		}
	}
	// Do this after to ensure all toolsets are enabled if "all" is present anywhere in list
	if tg.everythingOn {
		for name := range tg.Toolsets {
			err := tg.EnableToolset(name)
			if err != nil {
				return err
			}
		}
		return nil
	}
	return nil
}

func (tg *ToolsetGroup) EnableToolset(name string) error {
	toolset, exists := tg.Toolsets[name]
	if !exists {
		return fmt.Errorf("toolset %s does not exist", name)
	}
	toolset.Enabled = true
	tg.Toolsets[name] = toolset
	return nil
}

func (tg *ToolsetGroup) RegisterTools(s *server.MCPServer) {
	for _, toolset := range tg.Toolsets {
		toolset.RegisterTools(s)
	}
}
