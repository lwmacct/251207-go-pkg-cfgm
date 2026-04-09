package client

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
)

func cloneCommandForTest(t *testing.T) *cli.Command {
	t.Helper()

	cmd := *Command
	cmd.Writer = &bytes.Buffer{}
	cmd.ErrWriter = &bytes.Buffer{}
	cmd.ExitErrHandler = func(_ context.Context, _ *cli.Command, _ error) {}

	return &cmd
}

func commandOutput(t *testing.T, cmd *cli.Command) string {
	t.Helper()

	buf, ok := cmd.Writer.(*bytes.Buffer)
	require.True(t, ok)

	return buf.String()
}

func TestClientCommandShowsClientHelpWithoutSubcommand(t *testing.T) {
	cmd := cloneCommandForTest(t)

	err := cmd.Run(context.Background(), []string{"client"})
	require.NoError(t, err)

	output := commandOutput(t, cmd)
	assert.Contains(t, output, "client - HTTP 客户端工具")
	assert.Contains(t, output, "health")
	assert.Contains(t, output, "get")
}

func TestClientCommandRejectsUnknownSubcommand(t *testing.T) {
	cmd := cloneCommandForTest(t)

	err := cmd.Run(context.Background(), []string{"client", "healt"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `unknown client subcommand "healt"`)
	assert.Contains(t, err.Error(), "Did you mean")
	assert.Contains(t, err.Error(), "health")
}
