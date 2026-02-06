package ghmcp

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/github/github-mcp-server/pkg/github"
	mcplog "github.com/github/github-mcp-server/pkg/log"
	"github.com/github/github-mcp-server/pkg/translations"
	gogithub "github.com/google/go-github/v69/github"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/shurcooL/githubv4"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// createClient returns an appropriate GitHub client based on available authentication methods.
// It tries GitHub App authentication first, then falls back to PAT authentication.
func createClient(cfg MCPServerConfig) (*gogithub.Client, error) {
	// Try GitHub App authentication first
	appID := viper.GetInt64("app_id")
	installationID := viper.GetInt64("installation_id")

	// Check for private key - can be provided as file path or direct content
	privateKeyPath := viper.GetString("private_key_file_path")
	privateKeyContent := viper.GetString("private_key")

	// If we have the necessary GitHub App credentials
	if appID != 0 && installationID != 0 && (privateKeyPath != "" || privateKeyContent != "") {
		var itr *ghinstallation.Transport
		var err error

		// Create transport based on how the private key was provided
		if privateKeyContent != "" {
			// If private key content was provided directly
			// The content might be base64 encoded or have escaped newlines
			privateKeyContent = strings.ReplaceAll(privateKeyContent, "\\n", "\n")
			itr, err = ghinstallation.New(http.DefaultTransport, appID, installationID, []byte(privateKeyContent))
		} else {
			// If private key file path was provided
			itr, err = ghinstallation.NewKeyFromFile(http.DefaultTransport, appID, installationID, privateKeyPath)
		}

		if err != nil {
			return nil, fmt.Errorf("failed to create GitHub App transport: %w", err)
		}

		// Set the base URL if a custom host is specified
		if cfg.Host != "" {
			apiHost, err := parseAPIHost(cfg.Host)
			if err != nil {
				return nil, fmt.Errorf("failed to parse API host: %w", err)
			}
			itr.BaseURL = apiHost.baseRESTURL.String()
		}

		// Create client with the transport
		client := gogithub.NewClient(&http.Client{Transport: itr})
		client.UserAgent = fmt.Sprintf("github-mcp-server/%s", cfg.Version)

		return client, nil
	}

	// Fall back to PAT authentication
	token := cfg.Token
	if token == "" {
		return nil, fmt.Errorf("neither GitHub App credentials nor personal access token provided")
	}

	// Create client with PAT
	client := gogithub.NewClient(nil).WithAuthToken(token)
	client.UserAgent = fmt.Sprintf("github-mcp-server/%s", cfg.Version)

	// Set custom API URL if specified
	if cfg.Host != "" {
		apiHost, err := parseAPIHost(cfg.Host)
		if err != nil {
			return nil, fmt.Errorf("failed to parse API host: %w", err)
		}
		client.BaseURL = apiHost.baseRESTURL
		client.UploadURL = apiHost.uploadURL
	}

	return client, nil
}

// createGQLClient returns an appropriate GitHub GraphQL client based on available authentication methods.
func createGQLClient(cfg MCPServerConfig) (*githubv4.Client, *http.Client, error) {
	// Try GitHub App authentication first
	appID := viper.GetInt64("app_id")
	installationID := viper.GetInt64("installation_id")

	// Check for private key - can be provided as file path or direct content
	privateKeyPath := viper.GetString("private_key_file_path")
	privateKeyContent := viper.GetString("private_key")

	// If we have the necessary GitHub App credentials
	if appID != 0 && installationID != 0 && (privateKeyPath != "" || privateKeyContent != "") {
		var itr *ghinstallation.Transport
		var err error

		// Create transport based on how the private key was provided
		if privateKeyContent != "" {
			// If private key content was provided directly
			privateKeyContent = strings.ReplaceAll(privateKeyContent, "\\n", "\n")
			itr, err = ghinstallation.New(http.DefaultTransport, appID, installationID, []byte(privateKeyContent))
		} else {
			// If private key file path was provided
			itr, err = ghinstallation.NewKeyFromFile(http.DefaultTransport, appID, installationID, privateKeyPath)
		}

		if err != nil {
			return nil, nil, fmt.Errorf("failed to create GitHub App transport: %w", err)
		}

		// Set the base URL if a custom host is specified
		if cfg.Host != "" {
			apiHost, err := parseAPIHost(cfg.Host)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to parse API host: %w", err)
			}
			itr.BaseURL = apiHost.baseRESTURL.String()
		}

		// Create HTTP client with transport
		httpClient := &http.Client{Transport: itr}

		// Create GraphQL client
		var gqlClient *githubv4.Client
		if cfg.Host != "" {
			apiHost, err := parseAPIHost(cfg.Host)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to parse API host: %w", err)
			}
			gqlClient = githubv4.NewEnterpriseClient(apiHost.graphqlURL.String(), httpClient)
		} else {
			gqlClient = githubv4.NewClient(httpClient)
		}

		return gqlClient, httpClient, nil
	}

	// Fall back to PAT authentication
	token := cfg.Token
	if token == "" {
		return nil, nil, fmt.Errorf("neither GitHub App credentials nor personal access token provided")
	}

	// Create HTTP client with bearer auth transport
	httpClient := &http.Client{
		Transport: &bearerAuthTransport{
			transport: http.DefaultTransport,
			token:     token,
		},
	}

	// Create GraphQL client
	var gqlClient *githubv4.Client
	if cfg.Host != "" {
		apiHost, err := parseAPIHost(cfg.Host)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to parse API host: %w", err)
		}
		gqlClient = githubv4.NewEnterpriseClient(apiHost.graphqlURL.String(), httpClient)
	} else {
		gqlClient = githubv4.NewClient(httpClient)
	}

	return gqlClient, httpClient, nil
}

type MCPServerConfig struct {
	// Version of the server
	Version string

	// GitHub Host to target for API requests (e.g. github.com or github.enterprise.com)
	Host string

	// GitHub Token to authenticate with the GitHub API
	Token string

	// EnabledToolsets is a list of toolsets to enable
	// See: https://github.com/github/github-mcp-server?tab=readme-ov-file#tool-configuration
	EnabledToolsets []string

	// Whether to enable dynamic toolsets
	// See: https://github.com/github/github-mcp-server?tab=readme-ov-file#dynamic-tool-discovery
	DynamicToolsets bool

	// ReadOnly indicates if we should only offer read-only tools
	ReadOnly bool

	// Installations maps organization names to GitHub App installation IDs
	Installations map[string]int64

	// Translator provides translated text for the server tooling
	Translator translations.TranslationHelperFunc
}

func NewMCPServer(cfg MCPServerConfig) (*server.MCPServer, error) {
	// Create REST client
	restClient, err := createClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create GitHub REST client: %w", err)
	}

	// Create GraphQL client (for user agent hook only; actual GQL clients from factory)
	_, gqlHTTPClient, err := createGQLClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create GitHub GraphQL client: %w", err)
	}

	// When a client send an initialize request, update the user agent to include the client info.
	beforeInit := func(_ context.Context, _ any, message *mcp.InitializeRequest) {
		userAgent := fmt.Sprintf(
			"github-mcp-server/%s (%s/%s)",
			cfg.Version,
			message.Params.ClientInfo.Name,
			message.Params.ClientInfo.Version,
		)

		restClient.UserAgent = userAgent

		gqlHTTPClient.Transport = &userAgentTransport{
			transport: gqlHTTPClient.Transport,
			agent:     userAgent,
		}
	}

	hooks := &server.Hooks{
		OnBeforeInitialize: []server.OnBeforeInitializeFunc{beforeInit},
	}

	ghServer := github.NewServer(cfg.Version, server.WithHooks(hooks))

	enabledToolsets := cfg.EnabledToolsets
	if cfg.DynamicToolsets {
		// filter "all" from the enabled toolsets
		enabledToolsets = make([]string, 0, len(cfg.EnabledToolsets))
		for _, toolset := range cfg.EnabledToolsets {
			if toolset != "all" {
				enabledToolsets = append(enabledToolsets, toolset)
			}
		}
	}

	// Create multi-org client factory
	appID := viper.GetInt64("app_id")
	privateKey := []byte(viper.GetString("private_key"))

	clientFactory := github.NewMultiOrgClientFactory(
		appID,
		privateKey,
		cfg.Installations,
		cfg.Host,
		cfg.Version,
	)
	getClient := clientFactory.GetClientFn()
	getGQLClient := clientFactory.GetGQLClientFn()

	// Create default toolsets
	toolsets, err := github.InitToolsets(
		enabledToolsets,
		cfg.ReadOnly,
		getClient,
		getGQLClient,
		cfg.Translator,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize toolsets: %w", err)
	}

	github.RegisterResources(ghServer, getClient, cfg.Translator)

	// Register the tools with the server
	toolsets.RegisterTools(ghServer)

	if cfg.DynamicToolsets {
		dynamic := github.InitDynamicToolset(ghServer, toolsets, cfg.Translator)
		dynamic.RegisterTools(ghServer)
	}

	return ghServer, nil
}

type StdioServerConfig struct {
	// Version of the server
	Version string

	// GitHub Host to target for API requests (e.g. github.com or github.enterprise.com)
	Host string

	// GitHub Token to authenticate with the GitHub API
	Token string

	// EnabledToolsets is a list of toolsets to enable
	// See: https://github.com/github/github-mcp-server?tab=readme-ov-file#tool-configuration
	EnabledToolsets []string

	// Whether to enable dynamic toolsets
	// See: https://github.com/github/github-mcp-server?tab=readme-ov-file#dynamic-tool-discovery
	DynamicToolsets bool

	// ReadOnly indicates if we should only register read-only tools
	ReadOnly bool

	// ExportTranslations indicates if we should export translations
	// See: https://github.com/github/github-mcp-server?tab=readme-ov-file#i18n--overriding-descriptions
	ExportTranslations bool

	// EnableCommandLogging indicates if we should log commands
	EnableCommandLogging bool

	// Path to the log file if not stderr
	LogFilePath string

	// Installations maps organization names to GitHub App installation IDs
	Installations map[string]int64
}

// RunStdioServer is not concurrent safe.
func RunStdioServer(cfg StdioServerConfig) error {
	// Create app context
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	t, dumpTranslations := translations.TranslationHelper()

	ghServer, err := NewMCPServer(MCPServerConfig{
		Version:         cfg.Version,
		Host:            cfg.Host,
		Token:           cfg.Token,
		EnabledToolsets: cfg.EnabledToolsets,
		DynamicToolsets: cfg.DynamicToolsets,
		ReadOnly:        cfg.ReadOnly,
		Installations:   cfg.Installations,
		Translator:      t,
	})
	if err != nil {
		return fmt.Errorf("failed to create MCP server: %w", err)
	}

	stdioServer := server.NewStdioServer(ghServer)

	logrusLogger := logrus.New()
	if cfg.LogFilePath != "" {
		file, err := os.OpenFile(cfg.LogFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
		if err != nil {
			return fmt.Errorf("failed to open log file: %w", err)
		}

		logrusLogger.SetLevel(logrus.DebugLevel)
		logrusLogger.SetOutput(file)
	}
	stdLogger := log.New(logrusLogger.Writer(), "stdioserver", 0)
	stdioServer.SetErrorLogger(stdLogger)

	if cfg.ExportTranslations {
		// Once server is initialized, all translations are loaded
		dumpTranslations()
	}

	// Start listening for messages
	errC := make(chan error, 1)
	go func() {
		in, out := io.Reader(os.Stdin), io.Writer(os.Stdout)

		if cfg.EnableCommandLogging {
			loggedIO := mcplog.NewIOLogger(in, out, logrusLogger)
			in, out = loggedIO, loggedIO
		}

		errC <- stdioServer.Listen(ctx, in, out)
	}()

	// Output github-mcp-server string
	_, _ = fmt.Fprintf(os.Stderr, "GitHub MCP Server running on stdio\n")

	// Wait for shutdown signal
	select {
	case <-ctx.Done():
		logrusLogger.Infof("shutting down server...")
	case err := <-errC:
		if err != nil {
			return fmt.Errorf("error running server: %w", err)
		}
	}

	return nil
}

type apiHost struct {
	baseRESTURL *url.URL
	graphqlURL  *url.URL
	uploadURL   *url.URL
}

func newDotcomHost() (apiHost, error) {
	baseRestURL, err := url.Parse("https://api.github.com/")
	if err != nil {
		return apiHost{}, fmt.Errorf("failed to parse dotcom REST URL: %w", err)
	}

	gqlURL, err := url.Parse("https://api.github.com/graphql")
	if err != nil {
		return apiHost{}, fmt.Errorf("failed to parse dotcom GraphQL URL: %w", err)
	}

	uploadURL, err := url.Parse("https://uploads.github.com")
	if err != nil {
		return apiHost{}, fmt.Errorf("failed to parse dotcom Upload URL: %w", err)
	}

	return apiHost{
		baseRESTURL: baseRestURL,
		graphqlURL:  gqlURL,
		uploadURL:   uploadURL,
	}, nil
}

func newGHECHost(hostname string) (apiHost, error) {
	u, err := url.Parse(hostname)
	if err != nil {
		return apiHost{}, fmt.Errorf("failed to parse GHEC URL: %w", err)
	}

	// Unsecured GHEC would be an error
	if u.Scheme == "http" {
		return apiHost{}, fmt.Errorf("GHEC URL must be HTTPS")
	}

	restURL, err := url.Parse(fmt.Sprintf("https://api.%s/", u.Hostname()))
	if err != nil {
		return apiHost{}, fmt.Errorf("failed to parse GHEC REST URL: %w", err)
	}

	gqlURL, err := url.Parse(fmt.Sprintf("https://api.%s/graphql", u.Hostname()))
	if err != nil {
		return apiHost{}, fmt.Errorf("failed to parse GHEC GraphQL URL: %w", err)
	}

	uploadURL, err := url.Parse(fmt.Sprintf("https://uploads.%s", u.Hostname()))
	if err != nil {
		return apiHost{}, fmt.Errorf("failed to parse GHEC Upload URL: %w", err)
	}

	return apiHost{
		baseRESTURL: restURL,
		graphqlURL:  gqlURL,
		uploadURL:   uploadURL,
	}, nil
}

func newGHESHost(hostname string) (apiHost, error) {
	u, err := url.Parse(hostname)
	if err != nil {
		return apiHost{}, fmt.Errorf("failed to parse GHES URL: %w", err)
	}

	restURL, err := url.Parse(fmt.Sprintf("%s://%s/api/v3/", u.Scheme, u.Hostname()))
	if err != nil {
		return apiHost{}, fmt.Errorf("failed to parse GHES REST URL: %w", err)
	}

	gqlURL, err := url.Parse(fmt.Sprintf("%s://%s/api/graphql", u.Scheme, u.Hostname()))
	if err != nil {
		return apiHost{}, fmt.Errorf("failed to parse GHES GraphQL URL: %w", err)
	}

	uploadURL, err := url.Parse(fmt.Sprintf("%s://%s/api/uploads/", u.Scheme, u.Hostname()))
	if err != nil {
		return apiHost{}, fmt.Errorf("failed to parse GHES Upload URL: %w", err)
	}

	return apiHost{
		baseRESTURL: restURL,
		graphqlURL:  gqlURL,
		uploadURL:   uploadURL,
	}, nil
}

// Note that this does not handle ports yet, so development environments are out.
func parseAPIHost(s string) (apiHost, error) {
	if s == "" {
		return newDotcomHost()
	}

	u, err := url.Parse(s)
	if err != nil {
		return apiHost{}, fmt.Errorf("could not parse host as URL: %s", s)
	}

	if u.Scheme == "" {
		return apiHost{}, fmt.Errorf("host must have a scheme (http or https): %s", s)
	}

	if strings.HasSuffix(u.Hostname(), "github.com") {
		return newDotcomHost()
	}

	if strings.HasSuffix(u.Hostname(), "ghe.com") {
		return newGHECHost(s)
	}

	return newGHESHost(s)
}

type userAgentTransport struct {
	transport http.RoundTripper
	agent     string
}

func (t *userAgentTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("User-Agent", t.agent)
	return t.transport.RoundTrip(req)
}

type bearerAuthTransport struct {
	transport http.RoundTripper
	token     string
}

func (t *bearerAuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("Authorization", "Bearer "+t.token)
	return t.transport.RoundTrip(req)
}
