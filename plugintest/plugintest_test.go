package plugintest

import (
	"context"
	"os"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBinaryRoundTrip(t *testing.T) {
	ctx := context.Background()
	bin := Build(t, "testdata/echoplugin")

	meta, err := bin.Meta(ctx)
	require.NoError(t, err)
	assert.Equal(t, "echo", meta.ID)
	assert.False(t, meta.SupportsDataPreview, "echo has no DataFetcher")

	_, err = bin.Validate(ctx, pluginsdk.RawConfig{})
	require.Error(t, err, "missing required name must fail validation")

	validated, err := bin.Validate(ctx, pluginsdk.RawConfig{"name": "orders"})
	require.NoError(t, err)
	assert.Equal(t, "orders", validated["name"])

	// Discover runs in a fresh process; the echo plugin's Discover
	// reads state that only Validate sets, and its greeting comes from
	// an ApplyDefaults default. Both must survive the process model.
	result, err := bin.Discover(ctx, pluginsdk.RawConfig{"name": "orders"})
	require.NoError(t, err)
	require.Len(t, result.Assets, 1)
	assert.Equal(t, "orders", *result.Assets[0].Name)
	assert.Equal(t, "hello", result.Assets[0].Metadata["greeting"])

	// Discover with an invalid config must fail via the implicit
	// Validate, not panic inside Discover.
	_, err = bin.Discover(ctx, pluginsdk.RawConfig{})
	require.Error(t, err)

	// FetchSampleData on a non-DataFetcher plugin fails cleanly.
	_, _, err = bin.FetchSampleData(ctx, pluginsdk.RawConfig{"name": "orders"}, &pluginsdk.Asset{})
	require.Error(t, err)
}

func TestQueryRoundTrip(t *testing.T) {
	ctx := context.Background()
	bin := Build(t, "testdata/queryplugin")

	meta, err := bin.Meta(ctx)
	require.NoError(t, err)
	assert.True(t, meta.SupportsQuery, "Serve must detect the Querier")

	config := pluginsdk.RawConfig{"name": "warehouse"}
	plan, chunks, err := bin.Query(ctx, config, pluginsdk.QueryRequest{Statement: "SELECT * FROM orders", Identity: "user:alice"})
	require.NoError(t, err)
	require.NotNil(t, plan)
	assert.Equal(t, []string{"mrn://table/query/orders"}, plan.ReferencedMRNs)

	require.Len(t, chunks, 2)
	require.Len(t, chunks[0].Columns, 2, "the first chunk carries the schema")
	assert.Equal(t, "id", chunks[0].Columns[0].Name)
	assert.Empty(t, chunks[1].Columns)
	assert.Equal(t, "user:alice", chunks[1].Rows[0][1], "identity must reach the plugin")

	// A plugin that cannot plan a statement returns a nil plan, not an error.
	plan, _, err = bin.Query(ctx, config, pluginsdk.QueryRequest{Statement: "unplannable"})
	require.NoError(t, err)
	assert.Nil(t, plan)

	// Validation still runs before the query.
	_, _, err = bin.Query(ctx, pluginsdk.RawConfig{}, pluginsdk.QueryRequest{Statement: "SELECT 1"})
	require.Error(t, err)
}

func TestQueryUnsupported(t *testing.T) {
	ctx := context.Background()
	bin := Build(t, "testdata/echoplugin")

	meta, err := bin.Meta(ctx)
	require.NoError(t, err)
	assert.False(t, meta.SupportsQuery)

	_, _, err = bin.Query(ctx, pluginsdk.RawConfig{"name": "orders"}, pluginsdk.QueryRequest{Statement: "SELECT 1"})
	require.Error(t, err)
}

func TestOpenRefusesMissingChecksum(t *testing.T) {
	bin := Build(t, "testdata/echoplugin")

	_, err := pluginsdk.Open(bin.Path, nil, nil)
	require.Error(t, err)
}

// A binary replaced after the host hashed it must not run.
func TestOpenRefusesSwappedBinary(t *testing.T) {
	bin := Build(t, "testdata/echoplugin")
	sum, err := pluginsdk.HashBinary(bin.Path)
	require.NoError(t, err)

	other := Build(t, "testdata/queryplugin")
	data, err := os.ReadFile(other.Path)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(bin.Path, data, 0o755))

	_, err = pluginsdk.Open(bin.Path, sum, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "checksum")
}
