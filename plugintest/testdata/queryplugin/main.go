// Command queryplugin is a minimal Querier used by plugintest's own tests.
// It plans every statement as touching one fixed table and streams its rows
// back in two chunks, so the round-trip test covers planning, the column
// schema on the first chunk and multi-chunk streaming.
package main

import (
	"context"
	"fmt"
	"strings"

	pluginsdk "github.com/marmotdata/plugin-sdk"
)

type Config struct {
	pluginsdk.BaseConfig `json:",inline"`

	Name string `json:"name" validate:"required"`
}

type Source struct{}

func (s *Source) Validate(raw pluginsdk.RawConfig) (pluginsdk.RawConfig, error) {
	config, err := pluginsdk.UnmarshalConfig[Config](raw)
	if err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	if err := pluginsdk.ValidateStruct(config); err != nil {
		return nil, err
	}
	return raw, nil
}

func (s *Source) Discover(_ context.Context, _ pluginsdk.RawConfig) (*pluginsdk.DiscoveryResult, error) {
	return &pluginsdk.DiscoveryResult{}, nil
}

func (s *Source) PlanQuery(_ context.Context, _ pluginsdk.RawConfig, req pluginsdk.QueryRequest) (*pluginsdk.QueryPlan, error) {
	if strings.Contains(req.Statement, "unplannable") {
		return nil, nil
	}
	return &pluginsdk.QueryPlan{ReferencedMRNs: []string{"mrn://table/query/orders"}}, nil
}

func (s *Source) ExecuteQuery(_ context.Context, _ pluginsdk.RawConfig, req pluginsdk.QueryRequest, emit func(pluginsdk.QueryResultChunk) error) error {
	if err := emit(pluginsdk.QueryResultChunk{
		Columns: []pluginsdk.QueryColumn{{Name: "id", Type: "integer"}, {Name: "who", Type: "varchar"}},
		Rows:    [][]any{{1, req.Identity}},
	}); err != nil {
		return err
	}
	return emit(pluginsdk.QueryResultChunk{Rows: [][]any{{2, req.Identity}}})
}

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta: pluginsdk.Meta{
			ID:          "query",
			Name:        "Query",
			Description: "Test Querier plugin for plugintest",
			Icon:        "query",
			Category:    "test",
			ConfigSpec:  pluginsdk.GenerateConfigSpec(Config{}),
		},
		Source: &Source{},
	})
}
