package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/github/github-mcp-server/internal/ghmcp"
	"github.com/github/github-mcp-server/pkg/github"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// These variables are set by the build process using ldflags.
var version = "version"
var commit = "commit"
var date = "date"

var (
	rootCmd = &cobra.Command{
		Use:     "server",
		Short:   "GitHub MCP Server",
		Long:    `A GitHub MCP server that handles various tools and resources.`,
		Version: fmt.Sprintf("Version: %s\nCommit: %s\nBuild Date: %s", version, commit, date),
	}

	stdioCmd = &cobra.Command{
		Use:   "stdio",
		Short: "Start stdio server",
		Long:  `Start a server that communicates via standard input/output streams using JSON-RPC messages.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			// Check authentication configuration
			if err := validateAuthConfig(); err != nil {
				return err
			}

			token := viper.GetString("personal_access_token")

			// If you're wondering why we're not using viper.GetStringSlice("toolsets"),
			// it's because viper doesn't handle comma-separated values correctly for env
			// vars when using GetStringSlice.
			// https://github.com/spf13/viper/issues/380
			var enabledToolsets []string
			if err := viper.UnmarshalKey("toolsets", &enabledToolsets); err != nil {
				return fmt.Errorf("failed to unmarshal toolsets: %w", err)
			}

			// Parse multi-org installations
			installations := parseOrgInstallations()

			stdioServerConfig := ghmcp.StdioServerConfig{
				Version:              version,
				Host:                 viper.GetString("host"),
				Token:                token,
				EnabledToolsets:      enabledToolsets,
				DynamicToolsets:      viper.GetBool("dynamic_toolsets"),
				ReadOnly:             viper.GetBool("read-only"),
				WritePrivateOnly:     viper.GetBool("write-private-only"),
				ExportTranslations:   viper.GetBool("export-translations"),
				EnableCommandLogging: viper.GetBool("enable-command-logging"),
				LogFilePath:          viper.GetString("log-file"),
				Installations:        installations,
			}

			return ghmcp.RunStdioServer(stdioServerConfig)
		},
	}
)

// parseOrgInstallations parses GITHUB_INSTALLATION_ID_<ORG> environment variables
// and returns a map of organization name to installation ID.
// Also includes the default GITHUB_INSTALLATION_ID under "_default" key if set.
func parseOrgInstallations() map[string]int64 {
	installations := make(map[string]int64)
	prefix := "GITHUB_INSTALLATION_ID_"

	for _, env := range os.Environ() {
		if strings.HasPrefix(env, prefix) {
			parts := strings.SplitN(env, "=", 2)
			if len(parts) == 2 {
				org := strings.ToLower(strings.TrimPrefix(parts[0], prefix))
				org = strings.ReplaceAll(org, "_", "-") // Normalize underscores to dashes
				if id, err := strconv.ParseInt(parts[1], 10, 64); err == nil {
					installations[org] = id
				}
			}
		}
	}

	// Add default if set (for backwards compatibility)
	if defaultID := viper.GetInt64("installation_id"); defaultID != 0 {
		installations["_default"] = defaultID
	}

	return installations
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.SetVersionTemplate("{{.Short}}\n{{.Version}}\n")

	// Add global flags that will be shared by all commands
	rootCmd.PersistentFlags().StringSlice("toolsets", github.DefaultTools, "An optional comma separated list of groups of tools to allow with optional modes (e.g., 'repos:rw,issues:ro,users'), defaults to enabling all")
	rootCmd.PersistentFlags().Bool("dynamic-toolsets", false, "Enable dynamic toolsets")
	rootCmd.PersistentFlags().Bool("read-only", false, "Restrict the server to read-only operations")
	rootCmd.PersistentFlags().Bool("write-private-only", false, "Restrict all write operations to private repositories only")
	rootCmd.PersistentFlags().String("log-file", "", "Path to log file")
	rootCmd.PersistentFlags().Bool("enable-command-logging", false, "When enabled, the server will log all command requests and responses to the log file")
	rootCmd.PersistentFlags().Bool("export-translations", false, "Save translations to a JSON file")
	rootCmd.PersistentFlags().String("gh-host", "", "Specify the GitHub hostname (for GitHub Enterprise etc.)")

	// GitHub App authentication flags
	rootCmd.PersistentFlags().Int64("gh-app-id", 0, "GitHub App ID for authentication")
	rootCmd.PersistentFlags().Int64("gh-installation-id", 0, "GitHub App Installation ID for authentication")
	rootCmd.PersistentFlags().String("gh-private-key-path", "", "Path to GitHub App private key file")
	rootCmd.PersistentFlags().String("gh-private-key", "", "GitHub App private key content (alternative to private key file)")

	// Bind flag to viper
	_ = viper.BindPFlag("toolsets", rootCmd.PersistentFlags().Lookup("toolsets"))
	_ = viper.BindEnv("toolsets", "GITHUB_TOOLSETS")
	_ = viper.BindPFlag("dynamic_toolsets", rootCmd.PersistentFlags().Lookup("dynamic-toolsets"))
	_ = viper.BindPFlag("read-only", rootCmd.PersistentFlags().Lookup("read-only"))
	_ = viper.BindPFlag("write-private-only", rootCmd.PersistentFlags().Lookup("write-private-only"))
	_ = viper.BindPFlag("log-file", rootCmd.PersistentFlags().Lookup("log-file"))
	_ = viper.BindPFlag("enable-command-logging", rootCmd.PersistentFlags().Lookup("enable-command-logging"))
	_ = viper.BindPFlag("export-translations", rootCmd.PersistentFlags().Lookup("export-translations"))
	_ = viper.BindPFlag("host", rootCmd.PersistentFlags().Lookup("gh-host"))

	// Bind GitHub App authentication flags
	_ = viper.BindPFlag("app_id", rootCmd.PersistentFlags().Lookup("gh-app-id"))
	_ = viper.BindPFlag("installation_id", rootCmd.PersistentFlags().Lookup("gh-installation-id"))
	_ = viper.BindPFlag("private_key_file_path", rootCmd.PersistentFlags().Lookup("gh-private-key-path"))
	_ = viper.BindPFlag("private_key", rootCmd.PersistentFlags().Lookup("gh-private-key"))

	// Add subcommands
	rootCmd.AddCommand(stdioCmd)
}

func initConfig() {
	// Initialize Viper configuration
	viper.SetEnvPrefix("github")
	viper.AutomaticEnv()
}

// validateAuthConfig checks if either GitHub App authentication or PAT authentication is properly configured
func validateAuthConfig() error {
	// Check GitHub App authentication
	appID := viper.GetInt64("app_id")
	installationID := viper.GetInt64("installation_id")
	privateKeyPath := viper.GetString("private_key_file_path")
	privateKey := viper.GetString("private_key")

	// Check if GitHub App authentication is partially configured
	hasAppID := appID != 0
	hasInstallationID := installationID != 0
	hasPrivateKey := privateKeyPath != "" || privateKey != ""

	// Also check for multi-org installation IDs (GITHUB_INSTALLATION_ID_<ORG>)
	hasMultiOrgInstallations := len(parseOrgInstallations()) > 0
	hasAnyInstallation := hasInstallationID || hasMultiOrgInstallations

	if (hasAppID || hasAnyInstallation || hasPrivateKey) && !(hasAppID && hasAnyInstallation && hasPrivateKey) {
		return errors.New("incomplete GitHub App configuration: GITHUB_APP_ID, GITHUB_INSTALLATION_ID (or GITHUB_INSTALLATION_ID_<ORG>), and either GITHUB_PRIVATE_KEY_FILE_PATH or GITHUB_PRIVATE_KEY must all be set")
	}

	// Check PAT if GitHub App auth is not configured
	token := viper.GetString("personal_access_token")
	if !hasAppID && token == "" {
		return errors.New("no authentication method configured: either set GITHUB_PERSONAL_ACCESS_TOKEN or configure GitHub App authentication with GITHUB_APP_ID, GITHUB_INSTALLATION_ID (or GITHUB_INSTALLATION_ID_<ORG>), and GITHUB_PRIVATE_KEY_FILE_PATH or GITHUB_PRIVATE_KEY")
	}

	return nil
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
