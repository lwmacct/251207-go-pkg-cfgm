// Package templexp provides read-only variable interpolation for configuration
// strings.
//
// The syntax follows the interpolation subset used by Docker Compose. It
// supports ${VAR}, default values, alternate values, required values, nested
// interpolation in words, and $$ escaping. It does not execute commands or
// support assignment operators.
//
// Callers provide a [LookupFunc], so interpolation is independent of process
// environment state. Pass os.LookupEnv when environment variables are the
// desired source.
package templexp
