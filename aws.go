package pluginsdk

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// AWSCredentials configures AWS authentication for a plugin.
type AWSCredentials struct {
	UseDefault     bool   `json:"use_default,omitempty" description:"Use AWS credentials from environment or default profile (recommended)" default:"true"`
	ID             string `json:"id,omitempty" description:"AWS access key ID"`
	Secret         string `json:"secret,omitempty" description:"AWS secret access key" sensitive:"true"`
	Token          string `json:"token,omitempty" description:"AWS session token" sensitive:"true"`
	Profile        string `json:"profile,omitempty" description:"AWS profile to use from shared credentials file"`
	Role           string `json:"role,omitempty" description:"AWS IAM role ARN to assume"`
	RoleExternalID string `json:"role_external_id,omitempty" description:"External ID for cross-account role assumption"`
	Region         string `json:"region,omitempty" description:"AWS region for services"`
	Endpoint       string `json:"endpoint,omitempty" description:"Custom endpoint URL for AWS services" validate:"omitempty,url"`
}

// AWSConfig bundles AWSCredentials with common AWS-plugin options.
type AWSConfig struct {
	Credentials    AWSCredentials `json:"credentials" description:"AWS credentials configuration"`
	TagsToMetadata bool           `json:"tags_to_metadata,omitempty" description:"Convert AWS tags to Marmot metadata"`
	IncludeTags    []string       `json:"include_tags,omitempty" description:"List of AWS tags to include as metadata. By default, all tags are included."`
}

// NewAWSConfig loads an aws.Config honoring the credential chain,
// explicit keys, profile, role assumption, region and endpoint.
func (a *AWSConfig) NewAWSConfig(ctx context.Context) (aws.Config, error) {
	var opts []func(*config.LoadOptions) error

	if a.Credentials.Region != "" {
		opts = append(opts, config.WithRegion(a.Credentials.Region))
	}

	if a.Credentials.UseDefault || (a.Credentials.ID == "" && a.Credentials.Profile == "") {
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
