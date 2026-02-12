package toolsets

import (
	"fmt"
	"strings"
)

// ToolsetMode represents the access mode for a toolset
type ToolsetMode int

const (
	// ReadWrite allows both read and write tools
	ReadWrite ToolsetMode = iota
	// ReadOnly allows only read tools
	ReadOnly
)

// String returns the string representation of the toolset mode
func (m ToolsetMode) String() string {
	switch m {
	case ReadWrite:
		return "rw"
	case ReadOnly:
		return "ro"
	default:
		return "unknown"
	}
}

// ToolsetConfig represents the configuration for a single toolset
type ToolsetConfig struct {
	Name string
	Mode ToolsetMode
}

// ParseToolsetConfig parses a toolset configuration string like "repos:rw,issues:ro,users"
// and returns a slice of ToolsetConfig structs.
//
// Supported formats:
//   - "toolset" -> ToolsetConfig{Name: "toolset", Mode: ReadWrite}
//   - "toolset:rw" -> ToolsetConfig{Name: "toolset", Mode: ReadWrite}
//   - "toolset:ro" -> ToolsetConfig{Name: "toolset", Mode: ReadOnly}
//   - "toolset:readwrite" -> ToolsetConfig{Name: "toolset", Mode: ReadWrite}
//   - "toolset:readonly" -> ToolsetConfig{Name: "toolset", Mode: ReadOnly}
func ParseToolsetConfig(input string) ([]ToolsetConfig, error) {
	if input == "" {
		return []ToolsetConfig{}, nil
	}

	configs := []ToolsetConfig{}
	items := strings.Split(input, ",")

	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}

		if strings.Contains(item, ":") {
			parts := strings.Split(item, ":")
			if len(parts) != 2 {
				return nil, fmt.Errorf("invalid toolset format '%s': expected 'name:mode'", item)
			}

			name := strings.TrimSpace(parts[0])
			modeStr := strings.TrimSpace(parts[1])

			if name == "" {
				return nil, fmt.Errorf("invalid toolset format '%s': toolset name cannot be empty", item)
			}

			mode, err := parseMode(modeStr)
			if err != nil {
				return nil, fmt.Errorf("invalid mode '%s' for toolset '%s': %w", modeStr, name, err)
			}

			configs = append(configs, ToolsetConfig{Name: name, Mode: mode})
		} else {
			// Default to ReadWrite if no mode specified
			configs = append(configs, ToolsetConfig{Name: item, Mode: ReadWrite})
		}
	}

	return configs, nil
}

// parseMode parses a mode string and returns the corresponding ToolsetMode
func parseMode(modeStr string) (ToolsetMode, error) {
	switch strings.ToLower(modeStr) {
	case "rw", "readwrite":
		return ReadWrite, nil
	case "ro", "readonly":
		return ReadOnly, nil
	default:
		return ReadWrite, fmt.Errorf("supported modes are 'rw', 'readwrite', 'ro', 'readonly'")
	}
}

// ParseToolsetConfigFromSlice parses toolset configuration from a string slice
// (as typically provided by viper for command line flags)
func ParseToolsetConfigFromSlice(input []string) ([]ToolsetConfig, error) {
	if len(input) == 0 {
		return []ToolsetConfig{}, nil
	}

	// Join the slice with commas and parse as a single string
	// This handles both CLI flag usage and environment variable usage
	return ParseToolsetConfig(strings.Join(input, ","))
}
