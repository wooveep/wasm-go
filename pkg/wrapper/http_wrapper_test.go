package wrapper

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRedactSensitiveRequestHeaders(t *testing.T) {
	headers := [][2]string{
		{"Authorization", "Bearer secret"},
		{"x-api-key", "api-key-secret"},
		{"X-API-Key", "api-key-secret-2"},
		{"content-type", "application/json"},
	}

	redacted := redactSensitiveRequestHeaders(headers)

	require.Equal(t, [][2]string{
		{"Authorization", "[REDACTED]"},
		{"x-api-key", "[REDACTED]"},
		{"X-API-Key", "[REDACTED]"},
		{"content-type", "application/json"},
	}, redacted)
	require.Equal(t, "Bearer secret", headers[0][1])
	require.Equal(t, "api-key-secret", headers[1][1])
}
