package templexp_test

import (
	"strings"
	"testing"

	"github.com/lwmacct/251207-go-pkg-cfgm/pkg/templexp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExpand(t *testing.T) {
	variables := map[string]string{
		"EMPTY":   "",
		"SERVICE": "worker",
		"SET":     "set-value",
	}
	lookup := func(name string) (string, bool) {
		value, found := variables[name]

		return value, found
	}

	tests := []struct {
		name     string
		template string
		want     string
	}{
		{name: "value", template: `prefix-${SET}-suffix`, want: "prefix-set-value-suffix"},
		{name: "unset value", template: `x=${MISSING}`, want: "x="},
		{name: "default if unset", template: `${MISSING-default}`, want: "default"},
		{name: "default if unset preserves empty", template: `x=${EMPTY-default}`, want: "x="},
		{name: "default if empty", template: `${EMPTY:-default}`, want: "default"},
		{name: "alternate if set", template: `${EMPTY+alternate}`, want: "alternate"},
		{name: "alternate if unset", template: `${MISSING+alternate}`, want: ""},
		{name: "alternate if non-empty", template: `${SET:+alternate}`, want: "alternate"},
		{name: "alternate if empty", template: `${EMPTY:+alternate}`, want: ""},
		{name: "nested default", template: `${MISSING:-${SET}}`, want: "set-value"},
		{name: "lazy word", template: `${SET:-${MISSING:?not evaluated}}`, want: "set-value"},
		{name: "literal dollar", template: `$$${SET}`, want: "$set-value"},
		{name: "plain dollar", template: `$SET`, want: "$SET"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := templexp.Expand(tt.template, lookup)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExpandRequiredVariable(t *testing.T) {
	variables := map[string]string{"EMPTY": "", "SERVICE": "worker"}
	lookup := func(name string) (string, bool) {
		value, found := variables[name]

		return value, found
	}

	t.Run("unset", func(t *testing.T) {
		_, err := templexp.Expand(`${MISSING:?${SERVICE} requires MISSING}`, lookup)
		require.Error(t, err)

		var requiredErr *templexp.RequiredError
		require.ErrorAs(t, err, &requiredErr)
		assert.Equal(t, "MISSING", requiredErr.Name)
		assert.Equal(t, "worker requires MISSING", requiredErr.Message)
		assert.Equal(t, 0, requiredErr.Offset)
	})

	t.Run("empty with colon", func(t *testing.T) {
		_, err := templexp.Expand(`${EMPTY:?must not be empty}`, lookup)
		require.Error(t, err)
		assert.ErrorContains(t, err, "must not be empty")
	})

	t.Run("empty without colon", func(t *testing.T) {
		got, err := templexp.Expand(`${EMPTY?must be set}`, lookup)
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("default message", func(t *testing.T) {
		_, err := templexp.Expand(`${MISSING:?}`, lookup)
		require.Error(t, err)
		assert.ErrorContains(t, err, "required variable is unset or empty")
	})

	t.Run("default message without colon", func(t *testing.T) {
		_, err := templexp.Expand(`${MISSING?}`, lookup)
		require.Error(t, err)
		assert.ErrorContains(t, err, "required variable is unset")
	})
}

func TestExpandRejectsInvalidSyntax(t *testing.T) {
	lookup := func(string) (string, bool) { return "", false }
	tests := []struct {
		name     string
		template string
		offset   int
	}{
		{name: "empty name", template: `${}`, offset: 2},
		{name: "invalid name", template: `${1VAR}`, offset: 2},
		{name: "unclosed", template: `a${VAR`, offset: 1},
		{name: "assignment", template: `${VAR:=value}`, offset: 5},
		{name: "assignment without colon", template: `${VAR=value}`, offset: 5},
		{name: "unsupported operator", template: `${VAR:value}`, offset: 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := templexp.Expand(tt.template, lookup)
			require.Error(t, err)

			var syntaxErr *templexp.SyntaxError
			require.ErrorAs(t, err, &syntaxErr)
			assert.Equal(t, tt.offset, syntaxErr.Offset)
		})
	}
}

func TestExpandRejectsNilLookup(t *testing.T) {
	_, err := templexp.Expand(`${VAR}`, nil)
	require.EqualError(t, err, "templexp: nil lookup function")
}

func TestExpandLimitsNestingDepth(t *testing.T) {
	template := strings.Repeat(`${MISSING:-`, 101) + "value" + strings.Repeat("}", 101)
	_, err := templexp.Expand(template, func(string) (string, bool) { return "", false })
	require.Error(t, err)
	assert.ErrorContains(t, err, "maximum interpolation nesting depth exceeded")
}

func TestExpandCachesLookupResults(t *testing.T) {
	calls := 0
	lookup := func(string) (string, bool) {
		calls++

		return "value", true
	}

	got, err := templexp.Expand(`${VAR}-${VAR}`, lookup)
	require.NoError(t, err)
	assert.Equal(t, "value-value", got)
	assert.Equal(t, 1, calls)
}
