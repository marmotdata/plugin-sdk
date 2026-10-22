package pluginsdk

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPluginEnvDropsHostSecrets(t *testing.T) {
	env := pluginEnv([]string{
		"PATH=/usr/bin",
		"MARMOT_SERVER_ENCRYPTION_KEY=secret",
		"MARMOT_DATABASE_PASSWORD=secret",
		"MARMOT_API_KEY=secret",
		"AWS_ROLE_ARN=arn:aws:iam::1:role/x",
		"NOT_MARMOT_X=kept",
	})

	assert.Equal(t, []string{"PATH=/usr/bin", "AWS_ROLE_ARN=arn:aws:iam::1:role/x", "NOT_MARMOT_X=kept"}, env)
}

// configWriter stores its config in Validate, as most plugins do.
type configWriter struct{ config RawConfig }

func (c *configWriter) Validate(config RawConfig) (RawConfig, error) {
	c.config = config
	return config, nil
}

func (c *configWriter) Discover(context.Context, RawConfig) (*DiscoveryResult, error) {
	return &DiscoveryResult{}, nil
}

// A long-running query process takes concurrent calls for several sources; Validate must still run one call at a time, which -race checks.
func TestValidateRunsOneCallAtATime(t *testing.T) {
	s := &grpcServer{source: &configWriter{}}
	var wg sync.WaitGroup
	for i := range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.validate(RawConfig{"n": i})
			assert.NoError(t, err)
		}()
	}
	wg.Wait()
}
