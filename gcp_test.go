package pluginsdk

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractGCPConfig(t *testing.T) {
	cfg, err := ExtractGCPConfig(map[string]interface{}{
		"credentials": map[string]interface{}{
			"credentials_json": `{"type":"service_account"}`,
		},
	})
	require.NoError(t, err)
	assert.Equal(t, `{"type":"service_account"}`, cfg.Credentials.CredentialsJSON)
}

func TestGCPConfig_TokenSourceInvalidJSON(t *testing.T) {
	cfg := &GCPConfig{Credentials: GCPCredentials{CredentialsJSON: "{not valid}"}}
	_, err := cfg.TokenSource(context.Background(), "https://www.googleapis.com/auth/cloud-platform")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parsing GCP credentials")
}

func TestGCPConfig_TokenSourceMissingFile(t *testing.T) {
	cfg := &GCPConfig{Credentials: GCPCredentials{CredentialsFile: "/nonexistent/gcp-key.json"}}
	_, err := cfg.TokenSource(context.Background(), "https://www.googleapis.com/auth/cloud-platform")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reading GCP credentials file")
}
