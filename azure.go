package pluginsdk

import (
	"context"
	"fmt"
	"regexp"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
)

// azureDefaultAudience is the audience Entra expects on a federated
// identity credential's external token.
const azureDefaultAudience = "api://AzureADTokenExchange"

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// AzureCredentials configures how a plugin authenticates to Azure with
// an Entra identity. With tenant_id and client_id the pipeline presents
// a Marmot identity token as a client assertion to an app registration
// or user-assigned managed identity whose federated identity credential
// trusts the Marmot issuer; no secret is involved. With use_default the
// process's DefaultAzureCredential is used (managed identity, environment,
// CLI), for self-hosted servers on Azure. Plugins that also accept a
// connection string or an account key carry those fields themselves.
type AzureCredentials struct {
	TenantID   string `json:"tenant_id,omitempty" label:"Tenant ID" description:"Entra tenant of the identity the Marmot identity token is exchanged for. Set with client_id to federate: no secret is needed; add a federated credential trusting the Marmot issuer and the pipeline's subject on the Azure side"`
	ClientID   string `json:"client_id,omitempty" label:"Client ID" description:"Application (client) ID of the app registration or user-assigned managed identity whose federated credential trusts the Marmot issuer"`
	UseDefault bool   `json:"use_default,omitempty" description:"Use the Marmot process's own Azure credential (managed identity, environment, CLI)"`
}

// AzureConfig holds the Azure configuration shared across plugins,
// mirroring AWSConfig and GCPConfig. Embed it inline in a plugin's
// Config struct, together with Federation.
type AzureConfig struct {
	Credentials AzureCredentials `json:"credentials" description:"Azure credentials configuration"`
}

// Federate applies AzureCredentials.Federate to the embedded credentials.
func (a *AzureConfig) Federate(raw RawConfig) error {
	return a.Credentials.Federate(raw)
}

// Credential returns the token credential; see AzureCredentials.Credential.
func (a *AzureConfig) Credential(ctx context.Context) (azcore.TokenCredential, error) {
	return a.Credentials.Credential(ctx)
}

// Federated reports whether the credentials exchange a Marmot identity
// token. Both ids are needed; Federate enforces that.
func (c *AzureCredentials) Federated() bool {
	return c.TenantID != "" || c.ClientID != ""
}

// Federate checks the federation fields and derives the token audience
// into raw, the config Validate returns to the host. Tenant and client
// only make sense together, and an audience alone is refused: it is the
// host's signal to mint, which only an app registration can exchange.
func (c *AzureCredentials) Federate(raw RawConfig) error {
	audience := Audience(raw)
	if !c.Federated() {
		if audience != "" {
			return validationError("audience", "audience needs tenant_id and client_id; without them the plugin uses the credentials it names")
		}
		return nil
	}
	if c.TenantID == "" || c.ClientID == "" {
		field := "tenant_id"
		if c.TenantID != "" {
			field = "client_id"
		}
		return validationError(field, field+" is required with the other app registration field; set tenant_id and client_id together")
	}
	if !uuidPattern.MatchString(c.TenantID) {
		return validationError("tenant_id", "tenant_id must be the tenant's UUID")
	}
	if !uuidPattern.MatchString(c.ClientID) {
		return validationError("client_id", "client_id must be the application (client) UUID")
	}
	if c.UseDefault {
		return validationError("use_default", "use_default excludes tenant_id and client_id; a federated pipeline needs no ambient credential")
	}
	if audience == "" {
		audience = azureDefaultAudience
	}
	SetAudience(raw, audience)
	return nil
}

// Credential builds the token credential for the configured method.
// Federated credentials present the Marmot identity token the host left
// in IdentityTokenFile as a client assertion; Entra's SDK reads the
// file again at every token request, so a long run follows the host's
// refreshes. Neither method contacts Azure until the first token request.
func (c *AzureCredentials) Credential(ctx context.Context) (azcore.TokenCredential, error) {
	if !c.Federated() {
		cred, err := azidentity.NewDefaultAzureCredential(nil)
		if err != nil {
			return nil, fmt.Errorf("creating Azure credential: %w", err)
		}
		return cred, nil
	}
	file, ok := IdentityTokenFile(ctx)
	if !ok {
		return nil, ErrNoIdentityToken
	}
	cred, err := azidentity.NewWorkloadIdentityCredential(&azidentity.WorkloadIdentityCredentialOptions{
		TenantID:      c.TenantID,
		ClientID:      c.ClientID,
		TokenFilePath: file,
	})
	if err != nil {
		return nil, fmt.Errorf("creating Azure federated credential: %w", err)
	}
	return cred, nil
}
