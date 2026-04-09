package config

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfigClientURLIsAbsolute(t *testing.T) {
	cfg := DefaultConfig()

	parsed, err := url.Parse(cfg.Client.URL)
	require.NoError(t, err)
	assert.Equal(t, "http", parsed.Scheme)
	assert.Equal(t, "127.0.0.1:40117", parsed.Host)
}
