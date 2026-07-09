package plugintest

import (
	"context"
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
