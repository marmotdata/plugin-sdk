package pluginsdk

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testRoleARN = "arn:aws:iam::123456789012:role/marmot-s3"

func TestAWSFederateDerivesTheAudience(t *testing.T) {
	c := AWSCredentials{RoleARN: testRoleARN, Region: "eu-west-1"}
	raw := RawConfig{}
	require.NoError(t, c.Federate(raw))
	assert.Equal(t, "sts.amazonaws.com", Audience(raw))

	raw = RawConfig{"audience": "marmot-custom"}
	require.NoError(t, c.Federate(raw))
	assert.Equal(t, "marmot-custom", Audience(raw), "a registered client id is kept")
}

func TestAWSFederateWithoutRoleLeavesTheConfigAlone(t *testing.T) {
	c := AWSCredentials{UseDefault: true, Region: "eu-west-1"}
	raw := RawConfig{}
	require.NoError(t, c.Federate(raw))
	assert.False(t, Federated(raw))

	a := AWSConfig{Credentials: AWSCredentials{Profile: "prod"}}
	require.NoError(t, a.Federate(raw))
	assert.False(t, Federated(raw))
}

func TestAWSFederateRefusals(t *testing.T) {
	cases := []struct {
		name  string
		creds AWSCredentials
		raw   RawConfig
		field string
	}{
		{"audience alone", AWSCredentials{}, RawConfig{"audience": "sts.amazonaws.com"}, "audience"},
		{"not a role arn", AWSCredentials{RoleARN: "arn:aws:iam::123456789012:user/bob", Region: "eu-west-1"}, RawConfig{}, "role_arn"},
		{"role and static keys", AWSCredentials{RoleARN: testRoleARN, ID: "AKIA", Secret: "s", Region: "eu-west-1"}, RawConfig{}, "role_arn"},
		{"role and profile", AWSCredentials{RoleARN: testRoleARN, Profile: "prod", Region: "eu-west-1"}, RawConfig{}, "role_arn"},
		{"role without region", AWSCredentials{RoleARN: testRoleARN}, RawConfig{}, "region"},
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

func TestAWSNewConfigFederatedNeedsTheTokenFile(t *testing.T) {
	a := AWSConfig{Credentials: AWSCredentials{RoleARN: testRoleARN, Region: "eu-west-1"}}
	_, err := a.NewAWSConfig(context.Background())
	require.ErrorIs(t, err, ErrNoIdentityToken)
}

func TestAWSNewConfigFederatedBuildsWithoutContactingAWS(t *testing.T) {
	file := filepath.Join(t.TempDir(), "token")
	require.NoError(t, os.WriteFile(file, []byte("jwt"), 0o600))
	ctx := WithIdentityTokenFile(context.Background(), file)

	a := AWSConfig{Credentials: AWSCredentials{RoleARN: testRoleARN, Region: "eu-west-1"}}
	cfg, err := a.NewAWSConfig(ctx)
	require.NoError(t, err)
	assert.Equal(t, "eu-west-1", cfg.Region)
	require.NotNil(t, cfg.Credentials)

	// Chaining a second role on top keeps working.
	a.Credentials.Role = "arn:aws:iam::210987654321:role/reader"
	cfg, err = a.NewAWSConfig(ctx)
	require.NoError(t, err)
	require.NotNil(t, cfg.Credentials)
}
