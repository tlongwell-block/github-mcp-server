package github

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v69/github"
	"github.com/shurcooL/githubv4"
)

// RepoAccessType indicates how a repository should be accessed
type RepoAccessType int

const (
	// RepoAccessPrivate indicates the repository should be accessed with authentication
	RepoAccessPrivate RepoAccessType = iota
	// RepoAccessPublic indicates the repository should be accessed anonymously
	RepoAccessPublic
	// RepoAccessUnavailable indicates the repository is not accessible
	RepoAccessUnavailable
)

// accessCacheEntry represents a cached repository access result
type accessCacheEntry struct {
	accessType RepoAccessType
	expiry     time.Time
}

// RepoAccessCache caches repository access information
type RepoAccessCache struct {
	cache map[string]accessCacheEntry
	mu    sync.RWMutex
	ttl   time.Duration
}

// NewRepoAccessCache creates a new repository access cache
func NewRepoAccessCache(ttl time.Duration) *RepoAccessCache {
	return &RepoAccessCache{
		cache: make(map[string]accessCacheEntry),
		ttl:   ttl,
	}
}

// GetRepoAccessType determines how a repository should be accessed
func (c *RepoAccessCache) GetRepoAccessType(ctx context.Context, authClient *github.Client, owner, repo string) (RepoAccessType, error) {
	cacheKey := owner + "/" + repo

	// Check cache first
	c.mu.RLock()
	entry, found := c.cache[cacheKey]
	c.mu.RUnlock()

	if found && time.Now().Before(entry.expiry) {
		return entry.accessType, nil
	}

	// Cache miss or expired, determine access type
	accessType, err := determineRepoAccessType(ctx, authClient, owner, repo)
	if err != nil {
		return RepoAccessUnavailable, err
	}

	// Update cache
	c.mu.Lock()
	c.cache[cacheKey] = accessCacheEntry{
		accessType: accessType,
		expiry:     time.Now().Add(c.ttl),
	}
	c.mu.Unlock()

	return accessType, nil
}

// determineRepoAccessType checks how a repository should be accessed
func determineRepoAccessType(ctx context.Context, authClient *github.Client, owner, repo string) (RepoAccessType, error) {
	// First try with authenticated client
	_, resp, err := authClient.Repositories.Get(ctx, owner, repo)

	// If successful, we can access it with the authenticated client
	if err == nil {
		return RepoAccessPrivate, nil
	}

	// Check if it's a permission error (403) or not found error (404)
	if resp != nil && (resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusNotFound) {
		// Try anonymous access
		anonClient := github.NewClient(nil)
		_, anonResp, anonErr := anonClient.Repositories.Get(ctx, owner, repo)

		// If anonymous access succeeds, it's a public repo
		if anonErr == nil {
			return RepoAccessPublic, nil
		}

		// If anonymous access also fails with 404, repo doesn't exist
		if anonResp != nil && anonResp.StatusCode == http.StatusNotFound {
			return RepoAccessUnavailable, fmt.Errorf("repository %s/%s does not exist or is not accessible", owner, repo)
		}

		// Some other error with anonymous access
		return RepoAccessUnavailable, fmt.Errorf("failed to access repository anonymously: %w", anonErr)
	}

	// Some other error with authenticated access
	return RepoAccessUnavailable, fmt.Errorf("failed to access repository with authentication: %w", err)
}

// RepoAwareClientFactory creates clients based on repository access type
type RepoAwareClientFactory struct {
	authClient *github.Client
	cache      *RepoAccessCache
}

// NewRepoAwareClientFactory creates a new factory for repository-aware clients
func NewRepoAwareClientFactory(authClient *github.Client, cacheTTL time.Duration) *RepoAwareClientFactory {
	return &RepoAwareClientFactory{
		authClient: authClient,
		cache:      NewRepoAccessCache(cacheTTL),
	}
}

// GetClientForRepo returns the appropriate client for a repository
func (f *RepoAwareClientFactory) GetClientForRepo(ctx context.Context, owner, repo string) (*github.Client, error) {
	accessType, err := f.cache.GetRepoAccessType(ctx, f.authClient, owner, repo)
	if err != nil {
		return nil, err
	}

	switch accessType {
	case RepoAccessPrivate:
		return f.authClient, nil
	case RepoAccessPublic:
		return github.NewClient(nil), nil
	default:
		return nil, fmt.Errorf("repository %s/%s is not accessible", owner, repo)
	}
}

// WithRepoContext adds repository information to the context
func WithRepoContext(ctx context.Context, owner, repo string) context.Context {
	ctx = context.WithValue(ctx, "github.owner", owner)
	ctx = context.WithValue(ctx, "github.repo", repo)
	return ctx
}

// GetClientFn returns a function that gets the appropriate client based on context or owner
func (f *RepoAwareClientFactory) GetClientFn() GetClientFn {
	return func(ctx context.Context, owner string) (*github.Client, error) {
		// If owner not provided, try to get from context
		if owner == "" {
			ownerFromCtx, ok1 := ctx.Value("github.owner").(string)
			repo, ok2 := ctx.Value("github.repo").(string)

			if !ok1 || !ok2 {
				// If we can't determine the repo, use authenticated client
				return f.authClient, nil
			}

			return f.GetClientForRepo(ctx, ownerFromCtx, repo)
		}

		// Use provided owner
		return f.authClient, nil
	}
}

// MultiOrgClientFactory creates GitHub clients for different organizations
type MultiOrgClientFactory struct {
	appID          int64
	privateKey     []byte
	installations  map[string]int64 // org -> installation_id
	defaultInstall int64
	transports     map[string]*ghinstallation.Transport // cached per-org
	transportsMu   sync.RWMutex
	host           string
	version        string
}

// NewMultiOrgClientFactory creates a new multi-org client factory
func NewMultiOrgClientFactory(
	appID int64,
	privateKey []byte,
	installations map[string]int64,
	host, version string,
) *MultiOrgClientFactory {
	defaultInstall := installations["_default"]
	delete(installations, "_default")

	return &MultiOrgClientFactory{
		appID:          appID,
		privateKey:     privateKey,
		installations:  installations,
		defaultInstall: defaultInstall,
		transports:     make(map[string]*ghinstallation.Transport),
		host:           host,
		version:        version,
	}
}

// getInstallationID returns the installation ID for a given organization
func (f *MultiOrgClientFactory) getInstallationID(owner string) int64 {
	owner = strings.ToLower(owner)

	// Try exact match first
	if id, ok := f.installations[owner]; ok {
		return id
	}

	// Fall back to default
	return f.defaultInstall
}

// GetClientFn returns a function that gets the appropriate client based on owner
func (f *MultiOrgClientFactory) GetClientFn() GetClientFn {
	return func(ctx context.Context, owner string) (*github.Client, error) {
		if owner == "" {
			// User-scoped operations (GetMe) use default
			owner = "_default"
		}

		installID := f.getInstallationID(owner)
		if installID == 0 && len(f.installations) > 0 {
			for _, id := range f.installations {
				installID = id
				break
			}
		}
		if installID == 0 {
			client := github.NewClient(nil)
			client.UserAgent = fmt.Sprintf("github-mcp-server/%s", f.version)
			return client, nil
		}

		// Get or create transport for this installation
		transport, err := f.getOrCreateTransport(owner, installID)
		if err != nil {
			return nil, err
		}

		client := github.NewClient(&http.Client{Transport: transport})
		client.UserAgent = fmt.Sprintf("github-mcp-server/%s", f.version)
		return client, nil
	}
}

// GetGQLClientFn returns a function that gets the appropriate GraphQL client based on owner
func (f *MultiOrgClientFactory) GetGQLClientFn() GetGQLClientFn {
	return func(ctx context.Context, owner string) (*githubv4.Client, error) {
		if owner == "" {
			// User-scoped operations use default
			owner = "_default"
		}

		installID := f.getInstallationID(owner)
		if installID == 0 && len(f.installations) > 0 {
			for _, id := range f.installations {
				installID = id
				break
			}
		}
		if installID == 0 {
			return githubv4.NewClient(nil), nil
		}

		// Get or create transport for this installation
		transport, err := f.getOrCreateTransport(owner, installID)
		if err != nil {
			return nil, err
		}

		client := githubv4.NewClient(&http.Client{Transport: transport})
		return client, nil
	}
}

// getOrCreateTransport gets or creates a cached transport for an organization
func (f *MultiOrgClientFactory) getOrCreateTransport(org string, installID int64) (*ghinstallation.Transport, error) {
	f.transportsMu.RLock()
	if t, ok := f.transports[org]; ok {
		f.transportsMu.RUnlock()
		return t, nil
	}
	f.transportsMu.RUnlock()

	f.transportsMu.Lock()
	defer f.transportsMu.Unlock()

	// Double-check after acquiring write lock
	if t, ok := f.transports[org]; ok {
		return t, nil
	}

	transport, err := ghinstallation.New(http.DefaultTransport, f.appID, installID, f.privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create transport for %s: %w", org, err)
	}

	if f.host != "" {
		transport.BaseURL = f.host
	}

	f.transports[org] = transport
	return transport, nil
}
