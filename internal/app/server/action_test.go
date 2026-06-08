package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lwmacct/251207-go-pkg-cfgm/internal/config"
	"github.com/lwmacct/251207-go-pkg-cfgm/pkg/cfgm"
)

func TestNewMuxHealthEndpoint(t *testing.T) {
	mux := newMux(&config.Config{Server: config.DefaultConfig().Server})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"status":"ok"}`, rec.Body.String())
}

func TestNewMuxFallbackGetEndpoint(t *testing.T) {
	mux := newMux(&config.Config{Server: config.DefaultConfig().Server})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/path", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"message":"Hello, World!","path":"/path"}`, rec.Body.String())
}

func TestNewMuxServesFrontendDirectory(t *testing.T) {
	frontendDir := t.TempDir()
	err := os.WriteFile(filepath.Join(frontendDir, "index.html"), []byte("<h1>app</h1>"), 0600)
	require.NoError(t, err)

	cfg := config.DefaultConfig()
	cfg.Server.FrontendDir = frontendDir
	mux := newMux(&cfg)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "<h1>app</h1>")
}

func TestNewMuxKeepsAPIEndpointBeforeFrontendCatchAll(t *testing.T) {
	frontendDir := t.TempDir()
	err := os.WriteFile(filepath.Join(frontendDir, "path"), []byte("frontend path"), 0600)
	require.NoError(t, err)

	cfg := config.DefaultConfig()
	cfg.Server.FrontendDir = frontendDir
	mux := newMux(&cfg)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/path", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"message":"Hello, World!","path":"/path"}`, rec.Body.String())
}

func TestServerCommandIncludesRedisFlags(t *testing.T) {
	flagNames := map[string]bool{}
	for _, flag := range Command.Flags {
		for _, name := range flag.Names() {
			flagNames[name] = true
		}
	}

	assert.True(t, flagNames["redis.url"])
	assert.True(t, flagNames["redis.disabled"])
	assert.False(t, flagNames["redis.password"])
}

func TestServerCommandCoversConfigFlags(t *testing.T) {
	cfgm.AssertCommandFlagCoverage(
		t,
		Command,
		config.DefaultConfig(),
		[]string{"server", "redis"},
		cfgm.IgnoreConfigKeys("redis.password"),
	)
}
