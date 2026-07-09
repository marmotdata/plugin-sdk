package pluginsdk

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type DefaultsInner struct {
	Region string `json:"region" default:"us-east-1"`
}

type defaultsConfig struct {
	BaseConfig     `json:",inline"`
	*DefaultsInner `json:",inline"`

	Host      string   `json:"host"`
	Port      int      `json:"port" default:"8080"`
	Enabled   bool     `json:"enabled" default:"true"`
	Mode      string   `json:"mode" default:"production"`
	TLS       string   `json:"tls" default:"false"`
	Ratio     float64  `json:"ratio" default:"0.5"`
	Excludes  []string `json:"excludes" default:"[\"system\",\"jmx\"]"`
	NoDefault string   `json:"no_default"`
}

func TestApplyDefaults(t *testing.T) {
	t.Run("applies defaults for absent keys", func(t *testing.T) {
		config := &defaultsConfig{DefaultsInner: &DefaultsInner{}}
		ApplyDefaults(config, RawConfig{"host": "localhost"})

		assert.Equal(t, 8080, config.Port)
		assert.True(t, config.Enabled)
		assert.Equal(t, "production", config.Mode)
		assert.Equal(t, "false", config.TLS, "non-JSON-typed default applies verbatim to string fields")
		assert.Equal(t, 0.5, config.Ratio)
		assert.Equal(t, []string{"system", "jmx"}, config.Excludes)
		assert.Empty(t, config.NoDefault)
		assert.Empty(t, config.Host, "fields without defaults are untouched")
	})

	t.Run("present keys win over defaults", func(t *testing.T) {
		config := &defaultsConfig{Port: 9000, Enabled: false, Excludes: []string{}}
		ApplyDefaults(config, RawConfig{
			"port":     9000,
			"enabled":  false,
			"excludes": []interface{}{},
		})

		assert.Equal(t, 9000, config.Port)
		assert.False(t, config.Enabled, "explicit false is respected")
		assert.Empty(t, config.Excludes, "explicit empty list is respected")
	})

	t.Run("recurses into embedded sections", func(t *testing.T) {
		config := &defaultsConfig{DefaultsInner: &DefaultsInner{}}
		ApplyDefaults(config, RawConfig{})
		require.NotNil(t, config.DefaultsInner)
		assert.Equal(t, "us-east-1", config.Region)
	})

	t.Run("nil embedded pointer is skipped", func(t *testing.T) {
		config := &defaultsConfig{}
		assert.NotPanics(t, func() {
			ApplyDefaults(config, RawConfig{})
		})
	})

	t.Run("non-pointer and nil inputs are no-ops", func(t *testing.T) {
		assert.NotPanics(t, func() {
			ApplyDefaults(defaultsConfig{}, RawConfig{})
			ApplyDefaults(nil, RawConfig{})
		})
	})
}
