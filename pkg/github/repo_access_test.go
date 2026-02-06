package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/google/go-github/v69/github"
	"github.com/stretchr/testify/assert"
)

func TestRepoAccessCache_GetRepoAccessType(t *testing.T) {
	// Create a test server that responds differently based on the request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if this is an authenticated request
		authHeader := r.Header.Get("Authorization")

		switch r.URL.Path {
		case "/repos/owner/private-repo":
			if authHeader != "" {
				// Authenticated request to private repo succeeds
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"name":"private-repo","private":true}`))
			} else {
				// Unauthenticated request to private repo fails
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte(`{"message":"Not Found"}`))
			}
		case "/repos/owner/public-repo":
			if authHeader != "" {
				// Authenticated request to public repo fails with 403
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte(`{"message":"Resource not accessible by integration"}`))
			} else {
				// Unauthenticated request to public repo succeeds
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"name":"public-repo","private":false}`))
			}
		case "/repos/owner/nonexistent-repo":
			// Both authenticated and unauthenticated requests fail for nonexistent repo
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"message":"Not Found"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"message":"Not Found"}`))
		}
	}))
	defer server.Close()

	// Create an authenticated client
	authClient := github.NewClient(nil)
	authClient.BaseURL, _ = url.Parse(server.URL + "/")
	authClient = authClient.WithAuthToken("test-token")

	// Create the cache
	cache := NewRepoAccessCache(10 * time.Millisecond)

	// Test private repository
	accessType, err := cache.GetRepoAccessType(context.Background(), authClient, "owner", "private-repo")
	assert.NoError(t, err)
	assert.Equal(t, RepoAccessPrivate, accessType)

	// Test public repository
	accessType, err = cache.GetRepoAccessType(context.Background(), authClient, "owner", "public-repo")
	assert.NoError(t, err)
	assert.Equal(t, RepoAccessPublic, accessType) // Should be public because auth client fails but anon succeeds

	// Test nonexistent repository
	accessType, err = cache.GetRepoAccessType(context.Background(), authClient, "owner", "nonexistent-repo")
	assert.Error(t, err)
	assert.Equal(t, RepoAccessUnavailable, accessType)

	// Test caching
	// This should use the cached result without making a request
	accessType, err = cache.GetRepoAccessType(context.Background(), authClient, "owner", "private-repo")
	assert.NoError(t, err)
	assert.Equal(t, RepoAccessPrivate, accessType)

	// Wait for cache to expire
	time.Sleep(15 * time.Millisecond)

	// This should make a new request
	accessType, err = cache.GetRepoAccessType(context.Background(), authClient, "owner", "private-repo")
	assert.NoError(t, err)
	assert.Equal(t, RepoAccessPrivate, accessType)
}

func TestRepoAwareClientFactory_GetClientForRepo(t *testing.T) {
	// Create a test server that responds differently based on the request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if this is an authenticated request
		authHeader := r.Header.Get("Authorization")

		switch r.URL.Path {
		case "/repos/owner/private-repo":
			if authHeader != "" {
				// Authenticated request to private repo succeeds
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"name":"private-repo","private":true}`))
			} else {
				// Unauthenticated request to private repo fails
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte(`{"message":"Not Found"}`))
			}
		case "/repos/owner/public-repo":
			if authHeader != "" {
				// Authenticated request to public repo fails with 403
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte(`{"message":"Resource not accessible by integration"}`))
			} else {
				// Unauthenticated request to public repo succeeds
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"name":"public-repo","private":false}`))
			}
		case "/repos/owner/nonexistent-repo":
			// Both authenticated and unauthenticated requests fail for nonexistent repo
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"message":"Not Found"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"message":"Not Found"}`))
		}
	}))
	defer server.Close()

	// Create an authenticated client
	authClient := github.NewClient(nil)
	authClient.BaseURL, _ = url.Parse(server.URL + "/")
	authClient = authClient.WithAuthToken("test-token")

	// Create the factory
	factory := NewRepoAwareClientFactory(authClient, 10*time.Millisecond)

	// Test private repository
	client, err := factory.GetClientForRepo(context.Background(), "owner", "private-repo")
	assert.NoError(t, err)
	assert.Equal(t, authClient, client)

	// Test public repository
	client, err = factory.GetClientForRepo(context.Background(), "owner", "public-repo")
	assert.NoError(t, err)
	// Should be a different client than the auth client
	assert.NotEqual(t, authClient, client)

	// Test nonexistent repository
	client, err = factory.GetClientForRepo(context.Background(), "owner", "nonexistent-repo")
	assert.Error(t, err)
	assert.Nil(t, client)

	// Test context-based client selection
	ctx := WithRepoContext(context.Background(), "owner", "private-repo")
	client, err = factory.GetClientFn()(ctx, "")
	assert.NoError(t, err)
	assert.Equal(t, authClient, client)

	ctx = WithRepoContext(context.Background(), "owner", "public-repo")
	client, err = factory.GetClientFn()(ctx, "")
	assert.NoError(t, err)
	assert.NotEqual(t, authClient, client)

	// Test missing context info
	ctx = context.Background()
	client, err = factory.GetClientFn()(ctx, "")
	assert.NoError(t, err)
	assert.Equal(t, authClient, client) // Should default to auth client
}
