package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/lwmacct/251207-go-pkg-cfgm/internal/config"
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
