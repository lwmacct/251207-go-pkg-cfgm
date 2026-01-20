package templexp_test

import (
	"testing"

	"github.com/lwmacct/251207-go-pkg-cfgm/pkg/templexp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExpandTemplate_ShellParameterExpansion(t *testing.T) {
	t.Setenv("SHELL_SET", "set-value")
	t.Setenv("SHELL_EMPTY", "")

	tests := []struct {
		name     string
		template string
		want     string
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "basic expansion",
			template: `prefix-${SHELL_SET}-suffix`,
			want:     "prefix-set-value-suffix",
		},
		{
			name:     "missing expands to empty",
			template: `x=${SHELL_MISSING}`,
			want:     "x=",
		},
		{
			name:     "fallback with colon treats empty as unset",
			template: `${SHELL_EMPTY:-fallback}`,
			want:     "fallback",
		},
		{
			name:     "fallback without colon keeps empty",
			template: `x=${SHELL_EMPTY-fallback}`,
			want:     "x=",
		},
		{
			name:     "alternate with colon",
			template: `${SHELL_SET:+alt}`,
			want:     "alt",
		},
		{
			name:     "nested fallback",
			template: `${SHELL_MISSING:-${SHELL_SET}}`,
			want:     "set-value",
		},
		{
			name:     "assignment updates template data",
			template: `${SHELL_NEW:=value}-${SHELL_NEW}`,
			want:     "value-value",
		},
		{
			name:     "literal dollar",
			template: `$$${SHELL_SET}`,
			want:     "$set-value",
		},
		{
			name:     "required var triggers error",
			template: `${SHELL_MISSING:?missing}`,
			wantErr:  true,
			errMsg:   "missing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := templexp.ExpandTemplate(tt.template)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)

				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExpandTemplate_JSONConfig(t *testing.T) {
	t.Setenv("API_KEY", "sk-test-123")
	t.Setenv("MODEL", "gpt-4")

	jsonConfig := `{"name": "${AGENT_NAME:-test-agent}", "model": "${MODEL:-gpt-3.5-turbo}", "api_key": "${API_KEY}", "max_tokens": 2048}`

	expanded, err := templexp.ExpandTemplate(jsonConfig)
	require.NoError(t, err, "templexp.ExpandTemplate() should succeed")
	assert.NotEmpty(t, expanded, "templexp.ExpandTemplate() should return non-empty string")
	assert.Contains(t, expanded, "test-agent", "AGENT_NAME should fall back")
	assert.Contains(t, expanded, "gpt-4", "MODEL should be expanded to gpt-4")
	assert.Contains(t, expanded, "sk-test-123", "API_KEY should be expanded")
}
