package pluginsdk

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"sigs.k8s.io/yaml"
)

type AWSCredentials struct {
	UseDefault     bool   `json:"use_default,omitempty" description:"Use AWS credentials from environment or default profile (recommended)" default:"true"`
	ID             string `json:"id,omitempty" description:"AWS access key ID"`
	Secret         string `json:"secret,omitempty" description:"AWS secret access key" sensitive:"true"`
	Token          string `json:"token,omitempty" description:"AWS session token" sensitive:"true"`
	Profile        string `json:"profile,omitempty" description:"AWS profile to use from shared credentials file"`
	RoleARN        string `json:"role_arn,omitempty" label:"Role ARN (web identity)" description:"IAM role to assume with the Marmot identity token (AssumeRoleWithWebIdentity); its trust policy names the Marmot issuer as an OIDC provider. Setting it federates: no keys are needed, and region is required. role, if also set, is assumed on top of it"`
	Role           string `json:"role,omitempty" description:"AWS IAM role ARN to assume with AssumeRole from the base credentials (static keys, a profile, the default chain, or role_arn)"`
	RoleExternalID string `json:"role_external_id,omitempty" description:"External ID for cross-account role assumption"`
	Region         string `json:"region,omitempty" description:"AWS region for services"`
	Endpoint       string `json:"endpoint,omitempty" description:"Custom endpoint URL for AWS services" validate:"omitempty,url"`
}

type AWSConfig struct {
	Credentials    AWSCredentials `json:"credentials" description:"AWS credentials configuration"`
	TagsToMetadata bool           `json:"tags_to_metadata,omitempty" description:"Convert AWS tags to Marmot metadata"`
	IncludeTags    []string       `json:"include_tags,omitempty" description:"List of AWS tags to include as metadata. By default, all tags are included."`
}

func (a *AWSConfig) Validate() error {
	return nil
}

// awsDefaultAudience is what an IAM OIDC identity provider expects as
// client id unless registered with another.
const awsDefaultAudience = "sts.amazonaws.com"

// awsSessionName names the assumed-role session. CloudTrail records the
// token's subject alongside it as SubjectFromWebIdentityToken.
const awsSessionName = "marmot"

var awsRoleARNPattern = regexp.MustCompile(`^arn:aws[a-zA-Z-]*:iam::[0-9]{12}:role/.+$`)

// Federate applies AWSCredentials.Federate to the embedded credentials.
func (a *AWSConfig) Federate(raw RawConfig) error {
	return a.Credentials.Federate(raw)
}

// Federated reports whether the credentials exchange a Marmot identity
// token rather than use keys, a profile or the default chain.
func (c *AWSCredentials) Federated() bool {
	return c.RoleARN != ""
}

// Federate checks the federation fields and derives the token audience
// into raw, the config Validate returns to the host. Call it from
// Validate after UnmarshalConfig. An audience alone is refused: it is
// the host's signal to mint, which only a role can exchange. A role
// excludes static keys and a profile: the point is to have none.
func (c *AWSCredentials) Federate(raw RawConfig) error {
	audience := Audience(raw)
	if c.RoleARN == "" {
		if audience != "" {
			return validationError("audience", "audience needs role_arn; without a role the plugin uses the keys, profile or default chain it names")
		}
		return nil
	}
	if !awsRoleARNPattern.MatchString(c.RoleARN) {
		return validationError("role_arn", "role_arn must be an IAM role ARN: arn:aws:iam::<account>:role/<name>")
	}
	if c.ID != "" || c.Secret != "" || c.Token != "" || c.Profile != "" {
		return validationError("role_arn", "role_arn excludes static keys and a profile; a federated pipeline needs no key")
	}
	if c.Region == "" {
		return validationError("region", "region is required with role_arn; the STS exchange and the service calls are regional")
	}
	if audience == "" {
		audience = awsDefaultAudience
	}
	SetAudience(raw, audience)
	return nil
}

var ErrEndpointNotFound = fmt.Errorf("endpoint not found")

func ExtractAWSConfig(rawConfig map[string]interface{}) (*AWSConfig, error) {
	var awsCfg AWSConfig
	configBytes, err := yaml.Marshal(rawConfig)
	if err != nil {
		return nil, fmt.Errorf("marshaling raw config: %w", err)
	}

	if err := yaml.Unmarshal(configBytes, &awsCfg); err != nil {
		return nil, fmt.Errorf("unmarshaling into AWSConfig: %w", err)
	}

	return &awsCfg, nil
}

// DetectAWSCredentials checks if AWS credentials are available from environment or config files
func DetectAWSCredentials(ctx context.Context) *AWSCredentialStatus {
	status := &AWSCredentialStatus{
		Available: false,
		Sources:   []string{},
	}

	if os.Getenv("AWS_ACCESS_KEY_ID") != "" && os.Getenv("AWS_SECRET_ACCESS_KEY") != "" {
		status.Available = true
		status.Sources = append(status.Sources, "environment variables")
	}

	homeDir, err := os.UserHomeDir()
	if err == nil {
		credsPath := filepath.Join(homeDir, ".aws", "credentials")
		if _, err := os.Stat(credsPath); err == nil {
			status.Available = true
			status.Sources = append(status.Sources, "credentials file (~/.aws/credentials)")
		}
	}

	if err == nil {
		configPath := filepath.Join(homeDir, ".aws", "config")
		if _, err := os.Stat(configPath); err == nil {
			if !contains(status.Sources, "credentials file (~/.aws/credentials)") {
				status.Available = true
				status.Sources = append(status.Sources, "config file (~/.aws/config)")
			}
		}
	}

	if os.Getenv("AWS_CONTAINER_CREDENTIALS_RELATIVE_URI") != "" || os.Getenv("AWS_CONTAINER_CREDENTIALS_FULL_URI") != "" {
		status.Available = true
		status.Sources = append(status.Sources, "container credentials")
	}

	if status.Available {
		_, err := config.LoadDefaultConfig(ctx)
		if err != nil {
			status.Available = false
			status.Error = err.Error()
		}
	}

	return status
}

type AWSCredentialStatus struct {
	Available bool     `json:"available"`
	Sources   []string `json:"sources"`
	Error     string   `json:"error,omitempty"`
} // @name AWSCredentialStatus

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func (a *AWSConfig) NewAWSConfig(ctx context.Context) (aws.Config, error) {
	var opts []func(*config.LoadOptions) error

	if a.Credentials.Region != "" {
		opts = append(opts, config.WithRegion(a.Credentials.Region))
	}

	if a.Credentials.RoleARN != "" {
		// Federated: the default chain is not consulted, even if it
		// would resolve. The web identity provider goes in below, once
		// the config exists to build an STS client from.
		opts = append(opts, config.WithCredentialsProvider(aws.AnonymousCredentials{}))
	} else if a.Credentials.UseDefault || (a.Credentials.ID == "" && a.Credentials.Profile == "") {
		if a.Credentials.Profile != "" {
			opts = append(opts, config.WithSharedConfigProfile(a.Credentials.Profile))
		}
	} else {
		if a.Credentials.ID != "" && a.Credentials.Secret != "" {
			provider := credentials.NewStaticCredentialsProvider(
				a.Credentials.ID,
				a.Credentials.Secret,
				a.Credentials.Token,
			)
			opts = append(opts, config.WithCredentialsProvider(provider))
		}

		if a.Credentials.Profile != "" {
			opts = append(opts, config.WithSharedConfigProfile(a.Credentials.Profile))
		}
	}

	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return aws.Config{}, fmt.Errorf("loading AWS config: %w", err)
	}

	if a.Credentials.RoleARN != "" {
		provider, err := a.federatedCredentials(ctx, cfg)
		if err != nil {
			return aws.Config{}, err
		}
		cfg.Credentials = provider
	}

	if a.Credentials.Role != "" {
		stsClient := sts.NewFromConfig(cfg)
		assumeRoleOpts := func(o *stscreds.AssumeRoleOptions) {
			if a.Credentials.RoleExternalID != "" {
				o.ExternalID = aws.String(a.Credentials.RoleExternalID)
			}
		}

		provider := stscreds.NewAssumeRoleProvider(stsClient, a.Credentials.Role, assumeRoleOpts)
		cfg.Credentials = aws.NewCredentialsCache(provider)
	}

	if a.Credentials.Endpoint != "" {
		cfg.BaseEndpoint = aws.String(a.Credentials.Endpoint)
	}

	return cfg, nil
}

func ProcessAWSTags(tagsToMetadata bool, includeTags []string, tags map[string]string) map[string]interface{} {
	metadata := make(map[string]interface{})

	if !tagsToMetadata || len(tags) == 0 {
		return metadata
	}

	for key, value := range tags {
		if len(includeTags) > 0 {
			included := false
			for _, includeTag := range includeTags {
				if key == includeTag {
					included = true
					break
				}
			}
			if !included {
				continue
			}
		}

		metadata[fmt.Sprintf("tag_%s", key)] = value
	}

	return metadata
}

func ShouldIncludeResource(name string, filter Filter) bool {
	if len(filter.Include) == 0 && len(filter.Exclude) == 0 {
		return true
	}

	for _, pattern := range filter.Exclude {
		matched, err := regexp.MatchString(pattern, name)
		if err == nil && matched {
			return false
		}
	}

	if len(filter.Include) == 0 {
		return true
	}

	for _, pattern := range filter.Include {
		matched, err := regexp.MatchString(pattern, name)
		if err == nil && matched {
			return true
		}
	}

	return false
}

// federatedCredentials returns a cached provider that exchanges the
// Marmot identity token the host left in IdentityTokenFile for the
// role's credentials, and reads the file again at every refresh.
// AssumeRoleWithWebIdentity is unsigned, so the STS client runs
// anonymously.
func (a *AWSConfig) federatedCredentials(ctx context.Context, cfg aws.Config) (aws.CredentialsProvider, error) {
	file, ok := IdentityTokenFile(ctx)
	if !ok {
		return nil, ErrNoIdentityToken
	}
	stsCfg := cfg
	stsCfg.Credentials = aws.AnonymousCredentials{}
	provider := stscreds.NewWebIdentityRoleProvider(sts.NewFromConfig(stsCfg), a.Credentials.RoleARN, stscreds.IdentityTokenFile(file), func(o *stscreds.WebIdentityRoleOptions) {
		o.RoleSessionName = awsSessionName
	})
	return aws.NewCredentialsCache(provider), nil
}
