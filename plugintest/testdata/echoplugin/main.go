// Command echoplugin is a minimal plugin used by plugintest's own
// tests. Its Discover deliberately relies on state set by Validate, so
// the round-trip test proves the SDK runs Validate before Discover in
// each fresh plugin process.
package main

import (
	"context"
	"fmt"

	pluginsdk "github.com/marmotdata/plugin-sdk"
)

type Config struct {
	pluginsdk.BaseConfig `json:",inline"`

	Name     string `json:"name" validate:"required"`
	Greeting string `json:"greeting" default:"hello"`
}

type Source struct {
	config *Config
}

func (s *Source) Validate(raw pluginsdk.RawConfig) (pluginsdk.RawConfig, error) {
	config, err := pluginsdk.UnmarshalConfig[Config](raw)
	if err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	pluginsdk.ApplyDefaults(config, raw)

	if err := pluginsdk.ValidateStruct(config); err != nil {
		return nil, err
	}

	s.config = config
	return raw, nil
}

func (s *Source) Discover(_ context.Context, _ pluginsdk.RawConfig) (*pluginsdk.DiscoveryResult, error) {
	// Relies on s.config from Validate: nil here means the SDK failed
	// to run Validate first in this process.
	name := s.config.Name

	return &pluginsdk.DiscoveryResult{
		Assets: []pluginsdk.Asset{{
			Name:      &name,
			Type:      "Echo",
			Providers: []string{"Echo"},
			Metadata: map[string]interface{}{
				"greeting": s.config.Greeting,
			},
		}},
	}, nil
}

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta: pluginsdk.Meta{
			ID:          "echo",
			Name:        "Echo",
			Description: "Test plugin for plugintest",
			Icon:        "echo",
			Category:    "test",
			ConfigSpec:  pluginsdk.GenerateConfigSpec(Config{}),
		},
		Source: &Source{},
	})
}
