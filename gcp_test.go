package pluginsdk

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testProvider = "projects/123456789/locations/global/workloadIdentityPools/marmot/providers/marmot"

func TestGCPFederateDerivesTheAudience(t *testing.T) {
	for _, given := range []string{
		testProvider,
		"//iam.googleapis.com/" + testProvider,
		"https://iam.googleapis.com/" + testProvider,
	} {
		c := GCPCredentials{WorkloadIdentityProvider: given}
		raw := RawConfig{}
		require.NoError(t, c.Federate(raw), given)
		assert.Equal(t, testProvider, c.WorkloadIdentityProvider, "the provider is kept bare")
		assert.Equal(t, "https://iam.googleapis.com/"+testProvider, Audience(raw))
	}
}

func TestGCPFederateKeepsAnExplicitAudience(t *testing.T) {
	c := GCPCredentials{WorkloadIdentityProvider: testProvider}
	raw := RawConfig{"audience": "//iam.googleapis.com/" + testProvider}
	require.NoError(t, c.Federate(raw))
	assert.Equal(t, "//iam.googleapis.com/"+testProvider, Audience(raw))

	raw = RawConfig{"audience": "https://elsewhere.example"}
	err := c.Federate(raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "audience must start with")
}

func TestGCPFederateWithoutProviderLeavesTheConfigAlone(t *testing.T) {
	c := GCPCredentials{CredentialsJSON: "{}"}
	raw := RawConfig{"credentials": map[string]any{"credentials_json": "{}"}}
	require.NoError(t, c.Federate(raw))
	assert.False(t, Federated(raw))
}

func TestGCPFederateRefusals(t *testing.T) {
	cases := []struct {
		name  string
		creds GCPCredentials
		raw   RawConfig
		field string
	}{
		{"audience alone", GCPCredentials{}, RawConfig{"audience": "x"}, "audience"},
		{"service account alone", GCPCredentials{ServiceAccount: "sa@p.iam.gserviceaccount.com"}, RawConfig{}, "service_account"},
		{"provider by project id", GCPCredentials{WorkloadIdentityProvider: "projects/my-project/locations/global/workloadIdentityPools/p/providers/x"}, RawConfig{}, "workload_identity_provider"},
		{"provider and key json", GCPCredentials{WorkloadIdentityProvider: testProvider, CredentialsJSON: "{}"}, RawConfig{}, "workload_identity_provider"},
		{"provider and key file", GCPCredentials{WorkloadIdentityProvider: testProvider, CredentialsFile: "/k.json"}, RawConfig{}, "workload_identity_provider"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.creds.Federate(tc.raw)
			var verrs ValidationErrors
			require.ErrorAs(t, err, &verrs)
			require.Len(t, verrs.Errors, 1)
			assert.Equal(t, tc.field, verrs.Errors[0].Field)
		})
	}
}

func TestGCPTokenSourceFederatedNeedsTheTokenFile(t *testing.T) {
	c := GCPCredentials{WorkloadIdentityProvider: testProvider}
	_, err := c.TokenSource(context.Background())
	require.ErrorIs(t, err, ErrNoIdentityToken)
}

func TestGCPTokenSourceFederatedBuildsWithoutContactingGoogle(t *testing.T) {
	file := filepath.Join(t.TempDir(), "token")
	require.NoError(t, os.WriteFile(file, []byte("jwt"), 0o600))
	ctx := WithIdentityTokenFile(context.Background(), file)

	for _, c := range []GCPCredentials{
		{WorkloadIdentityProvider: testProvider},
		{WorkloadIdentityProvider: "//iam.googleapis.com/" + testProvider, ServiceAccount: "sa@p.iam.gserviceaccount.com"},
	} {
		ts, err := c.TokenSource(ctx)
		require.NoError(t, err)
		assert.NotNil(t, ts)
	}

	g := GCPConfig{Credentials: GCPCredentials{WorkloadIdentityProvider: testProvider}}
	ts, err := g.TokenSource(ctx, "https://www.googleapis.com/auth/bigquery.readonly")
	require.NoError(t, err)
	assert.NotNil(t, ts)
}
