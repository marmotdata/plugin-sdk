package pluginsdk

import (
	"context"
	"fmt"
	"os"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"sigs.k8s.io/yaml"
)

// GCPCredentials configures how a plugin authenticates to Google Cloud.
// With neither field set, Application Default Credentials are used
// (Workload Identity, a Cloud Run/GCE service account, or
// GOOGLE_APPLICATION_CREDENTIALS).
type GCPCredentials struct {
	CredentialsJSON string `json:"credentials_json,omitempty" description:"Service account key JSON content" sensitive:"true"`
	CredentialsFile string `json:"credentials_file,omitempty" description:"Path to a service account key JSON file"`
}

// GCPConfig holds the Google Cloud configuration shared across plugins,
// mirroring AWSConfig. Embed it inline in a plugin's Config struct.
type GCPConfig struct {
	Credentials GCPCredentials `json:"credentials" description:"GCP credentials configuration"`
}

func (g *GCPConfig) Validate() error {
	return nil
}

// ExtractGCPConfig pulls the GCP configuration out of a raw plugin config.
func ExtractGCPConfig(rawConfig map[string]interface{}) (*GCPConfig, error) {
	var cfg GCPConfig
	configBytes, err := yaml.Marshal(rawConfig)
	if err != nil {
		return nil, fmt.Errorf("marshaling raw config: %w", err)
	}
	if err := yaml.Unmarshal(configBytes, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshaling into GCPConfig: %w", err)
	}
	return &cfg, nil
}

// TokenSource returns an OAuth2 token source for the given scopes. It
// uses an explicit service account key (JSON content or file) when
// provided, otherwise Application Default Credentials (Workload
// Identity, a Cloud Run/GCE service account, or GOOGLE_APPLICATION_CREDENTIALS).
func (g *GCPConfig) TokenSource(ctx context.Context, scopes ...string) (oauth2.TokenSource, error) {
	keyJSON := []byte(g.Credentials.CredentialsJSON)
	if len(keyJSON) == 0 && g.Credentials.CredentialsFile != "" {
		data, err := os.ReadFile(g.Credentials.CredentialsFile)
		if err != nil {
			return nil, fmt.Errorf("reading GCP credentials file: %w", err)
		}
		keyJSON = data
	}

	if len(keyJSON) > 0 {
		// JWTConfigFromJSON is the non-deprecated path for a service
		// account key.
		jwtConfig, err := google.JWTConfigFromJSON(keyJSON, scopes...)
		if err != nil {
			return nil, fmt.Errorf("parsing GCP credentials: %w", err)
		}
		return jwtConfig.TokenSource(ctx), nil
	}

	ts, err := google.DefaultTokenSource(ctx, scopes...)
	if err != nil {
		return nil, fmt.Errorf("loading Google credentials: %w", err)
	}
	return ts, nil
}
