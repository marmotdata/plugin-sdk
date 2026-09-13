package pluginsdk

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testTenantID = "11111111-2222-3333-4444-555555555555"
	testClientID = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
)

func TestAzureFederateDerivesTheAudience(t *testing.T) {
	c := AzureCredentials{TenantID: testTenantID, ClientID: testClientID}
	raw := RawConfig{}
	require.NoError(t, c.Federate(raw))
	assert.Equal(t, "api://AzureADTokenExchange", Audience(raw))

	raw = RawConfig{"audience": "api://marmot"}
	require.NoError(t, c.Federate(raw))
	assert.Equal(t, "api://marmot", Audience(raw))
}

func TestAzureFederateWithoutIdsLeavesTheConfigAlone(t *testing.T) {
	raw := RawConfig{}
	require.NoError(t, (&AzureCredentials{UseDefault: true}).Federate(raw))
	assert.False(t, Federated(raw))
	require.NoError(t, (&AzureConfig{}).Federate(raw))
	assert.False(t, Federated(raw))
}

func TestAzureFederateRefusals(t *testing.T) {
	cases := []struct {
		name  string
		creds AzureCredentials
		raw   RawConfig
		field string
	}{
		{"audience alone", AzureCredentials{}, RawConfig{"audience": "api://AzureADTokenExchange"}, "audience"},
		{"tenant without client", AzureCredentials{TenantID: testTenantID}, RawConfig{}, "client_id"},
		{"client without tenant", AzureCredentials{ClientID: testClientID}, RawConfig{}, "tenant_id"},
		{"tenant not a uuid", AzureCredentials{TenantID: "contoso", ClientID: testClientID}, RawConfig{}, "tenant_id"},
		{"client not a uuid", AzureCredentials{TenantID: testTenantID, ClientID: "app"}, RawConfig{}, "client_id"},
		{"federated and default", AzureCredentials{TenantID: testTenantID, ClientID: testClientID, UseDefault: true}, RawConfig{}, "use_default"},
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

func TestAzureCredentialFederatedNeedsTheTokenFile(t *testing.T) {
	c := AzureCredentials{TenantID: testTenantID, ClientID: testClientID}
	_, err := c.Credential(context.Background())
	require.ErrorIs(t, err, ErrNoIdentityToken)
}

func TestAzureCredentialFederatedBuildsWithoutContactingEntra(t *testing.T) {
	file := filepath.Join(t.TempDir(), "token")
	require.NoError(t, os.WriteFile(file, []byte("jwt"), 0o600))
	ctx := WithIdentityTokenFile(context.Background(), file)

	a := AzureConfig{Credentials: AzureCredentials{TenantID: testTenantID, ClientID: testClientID}}
	cred, err := a.Credential(ctx)
	require.NoError(t, err)
	assert.NotNil(t, cred)
}
