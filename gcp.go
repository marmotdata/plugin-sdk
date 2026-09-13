package pluginsdk

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"golang.org/x/oauth2/google/externalaccount"
)

const (
	gcpSTSTokenURL = "https://sts.googleapis.com/v1/token" //nolint:gosec // G101: a URL, not a credential
	// gcpSTSAudiencePrefix prefixes the provider in the STS exchange request.
	gcpSTSAudiencePrefix = "//iam.googleapis.com/"
	// gcpTokenAudiencePrefix prefixes the provider in the aud claim a
	// provider accepts by default.
	gcpTokenAudiencePrefix = "https://iam.googleapis.com/" //nolint:gosec // G101: a URL, not a credential
	gcpJWTSubjectTokenType = "urn:ietf:params:oauth:token-type:jwt"
	gcpCloudPlatformScope  = "https://www.googleapis.com/auth/cloud-platform"
	gcpImpersonationURLFmt = "https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/%s:generateAccessToken"
	gcpProviderDescription = "projects/<number>/locations/global/workloadIdentityPools/<pool>/providers/<provider>"
)

// gcpProviderPattern is a Workload Identity provider. The project is its
// number, not its id; STS does not resolve ids.
var gcpProviderPattern = regexp.MustCompile(`^projects/[0-9]+/locations/global/workloadIdentityPools/[^/]+/providers/[^/]+$`)

// GCPCredentials configures how a plugin authenticates to Google Cloud.
// One of: a service account key (JSON content or file), a Workload
// Identity Federation provider (the pipeline's Marmot identity is
// exchanged for a Google credential, no key involved), or, with none
// set, Application Default Credentials (Workload Identity on GKE, a
// Cloud Run/GCE service account, or GOOGLE_APPLICATION_CREDENTIALS).
type GCPCredentials struct {
	CredentialsJSON          string `json:"credentials_json,omitempty" description:"Service account key JSON content" sensitive:"true"`
	CredentialsFile          string `json:"credentials_file,omitempty" description:"Path to a service account key JSON file"`
	WorkloadIdentityProvider string `json:"workload_identity_provider,omitempty" label:"Workload Identity Provider" description:"Workload Identity Federation provider to exchange the Marmot identity token at, projects/<number>/locations/global/workloadIdentityPools/<pool>/providers/<provider>. Setting it federates: the pipeline presents a Marmot identity token and no key is needed; bind the pipeline's subject on the Google Cloud side"`
	ServiceAccount           string `json:"service_account,omitempty" label:"Service Account" description:"Service account to impersonate after the exchange; empty acts as the federated principal directly"`
}

// GCPConfig holds the Google Cloud configuration shared across plugins,
// mirroring AWSConfig. Embed it inline in a plugin's Config struct,
// together with Federation when the plugin should be able to federate.
type GCPConfig struct {
	Credentials GCPCredentials `json:"credentials" description:"GCP credentials configuration"`
}

func (g *GCPConfig) Validate() error {
	return nil
}

// Federate applies GCPCredentials.Federate to the embedded credentials.
func (g *GCPConfig) Federate(raw RawConfig) error {
	return g.Credentials.Federate(raw)
}

// TokenSource returns an OAuth2 token source for the given scopes; see
// GCPCredentials.TokenSource.
func (g *GCPConfig) TokenSource(ctx context.Context, scopes ...string) (oauth2.TokenSource, error) {
	return g.Credentials.TokenSource(ctx, scopes...)
}

// Federated reports whether the credentials exchange a Marmot identity
// token rather than use a key or default credentials.
func (c *GCPCredentials) Federated() bool {
	return c.WorkloadIdentityProvider != ""
}

// Federate checks the federation fields and derives the token audience
// into raw, the config Validate returns to the host. A provider is
// accepted with or without its //iam.googleapis.com/ prefix and kept
// bare. An audience alone is refused: it is the host's signal to mint,
// which only a provider can exchange. A provider excludes a key.
func (c *GCPCredentials) Federate(raw RawConfig) error {
	c.WorkloadIdentityProvider = bareGCPProvider(c.WorkloadIdentityProvider)
	audience := Audience(raw)
	if c.WorkloadIdentityProvider == "" {
		if audience != "" {
			return validationError("audience", "audience needs workload_identity_provider; without a provider the plugin uses the key or default credentials it names")
		}
		if c.ServiceAccount != "" {
			return validationError("service_account", "service_account needs workload_identity_provider; it names the account to impersonate after the exchange")
		}
		return nil
	}
	if !gcpProviderPattern.MatchString(c.WorkloadIdentityProvider) {
		return validationError("workload_identity_provider", "workload_identity_provider must name a provider as "+gcpProviderDescription)
	}
	if c.CredentialsJSON != "" || c.CredentialsFile != "" {
		return validationError("workload_identity_provider", "workload_identity_provider excludes credentials_json and credentials_file; a federated pipeline needs no key")
	}
	if audience == "" {
		audience = gcpTokenAudiencePrefix + c.WorkloadIdentityProvider
	} else if !strings.HasPrefix(audience, gcpTokenAudiencePrefix) && !strings.HasPrefix(audience, gcpSTSAudiencePrefix) {
		return validationError("audience", "audience must start with "+gcpTokenAudiencePrefix+" or "+gcpSTSAudiencePrefix)
	}
	SetAudience(raw, audience)
	return nil
}

func bareGCPProvider(s string) string {
	s = strings.TrimPrefix(s, gcpTokenAudiencePrefix)
	return strings.TrimPrefix(s, gcpSTSAudiencePrefix)
}

// TokenSource returns an OAuth2 token source for the given scopes,
// cloud-platform when none are given. Federated credentials exchange the
// token in IdentityTokenFile at Google STS, impersonating ServiceAccount
// when set; the file is read at every exchange, so a long run follows
// the host's refreshes. Otherwise a key (JSON content or file) is used
// when given, and Application Default Credentials when not.
func (c *GCPCredentials) TokenSource(ctx context.Context, scopes ...string) (oauth2.TokenSource, error) {
	if len(scopes) == 0 {
		scopes = []string{gcpCloudPlatformScope}
	}

	if c.Federated() {
		file, ok := IdentityTokenFile(ctx)
		if !ok {
			return nil, ErrNoIdentityToken
		}
		exchange := externalaccount.Config{
			Audience:         gcpSTSAudiencePrefix + bareGCPProvider(c.WorkloadIdentityProvider),
			SubjectTokenType: gcpJWTSubjectTokenType,
			TokenURL:         gcpSTSTokenURL,
			Scopes:           scopes,
			CredentialSource: &externalaccount.CredentialSource{
				File:   file,
				Format: externalaccount.Format{Type: "text"},
			},
		}
		if c.ServiceAccount != "" {
			exchange.ServiceAccountImpersonationURL = fmt.Sprintf(gcpImpersonationURLFmt, c.ServiceAccount)
		}
		ts, err := externalaccount.NewTokenSource(ctx, exchange)
		if err != nil {
			return nil, fmt.Errorf("configuring workload identity federation: %w", err)
		}
		return ts, nil
	}

	keyJSON := []byte(c.CredentialsJSON)
	if len(keyJSON) == 0 && c.CredentialsFile != "" {
		data, err := os.ReadFile(c.CredentialsFile)
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
