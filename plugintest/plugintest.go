// Package plugintest helps plugin modules test their own built binary
// end to end, over the same wire protocol the Marmot host uses. Each
// call spawns the plugin process, performs one gRPC call, and kills the
// process again, exactly the host's short-lived process model, so
// tests catch state that leaks between Validate and Discover.
//
//	func TestDiscover(t *testing.T) {
//	    bin := plugintest.Build(t, "..") // plugin main package
//	    result, err := bin.Discover(context.Background(), pluginsdk.RawConfig{...})
//	    ...
//	}
package plugintest

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/hashicorp/go-hclog"
	pluginsdk "github.com/marmotdata/plugin-sdk"
)

// Binary is a built plugin binary under test.
type Binary struct {
	Path string
}

// Build compiles the plugin main package in pkgDir into a temporary
// binary and returns it. The binary is removed when the test finishes.
func Build(t testing.TB, pkgDir string) Binary {
	t.Helper()

	out := filepath.Join(t.TempDir(), "plugin-under-test")
	cmd := exec.Command("go", "build", "-o", out, ".")
	cmd.Dir = pkgDir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("building plugin binary in %s: %v\n%s", pkgDir, err, output)
	}

	return Binary{Path: out}
}

// Meta launches the binary and returns its metadata.
func (b Binary) Meta(ctx context.Context) (*pluginsdk.Meta, error) {
	process, err := b.open()
	if err != nil {
		return nil, err
	}
	defer process.Kill()

	return process.Source.GetMeta(ctx)
}

// Validate launches the binary and runs Validate over the wire.
func (b Binary) Validate(ctx context.Context, config pluginsdk.RawConfig) (pluginsdk.RawConfig, error) {
	process, err := b.open()
	if err != nil {
		return nil, err
	}
	defer process.Kill()

	return process.Source.Validate(ctx, config)
}

// Discover launches the binary and runs Discover over the wire.
func (b Binary) Discover(ctx context.Context, config pluginsdk.RawConfig) (*pluginsdk.DiscoveryResult, error) {
	process, err := b.open()
	if err != nil {
		return nil, err
	}
	defer process.Kill()

	return process.Source.Discover(ctx, config)
}

// FetchSampleData launches the binary and runs FetchSampleData over the
// wire. Plugins whose Source does not implement DataFetcher fail with
// an Unimplemented gRPC status.
func (b Binary) FetchSampleData(ctx context.Context, config pluginsdk.RawConfig, a *pluginsdk.Asset) ([]string, [][]interface{}, error) {
	process, err := b.open()
	if err != nil {
		return nil, nil, err
	}
	defer process.Kill()

	return process.Source.FetchSampleData(ctx, config, a)
}

func (b Binary) open() (*pluginsdk.PluginProcess, error) {
	return pluginsdk.Open(b.Path, hclog.NewNullLogger())
}
