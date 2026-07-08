package pluginsdk

import (
	"context"

	goplugin "github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"

	"github.com/marmotdata/plugin-sdk/proto"
)

// ProtocolVersion is the Marmot plugin protocol version. Bump it on
// breaking changes to the wire protocol; the go-plugin handshake
// rejects plugins built against a different version.
const ProtocolVersion = 1

// PluginSetName is the key under which the source plugin is registered
// in the go-plugin plugin map.
const PluginSetName = "source"

// Handshake is the shared handshake config between the Marmot host and
// its plugins. The magic cookie is not a security measure; it only
// prevents plugin binaries from being executed as normal CLIs by
// mistake.
var Handshake = goplugin.HandshakeConfig{
	ProtocolVersion:  ProtocolVersion,
	MagicCookieKey:   "MARMOT_PLUGIN",
	MagicCookieValue: "8b1c9e2f4a7d4e6b9c3f5a8d2e7b1c4f",
}

// RemoteSource is the host-facing view of a plugin process. It mirrors
// Source but is context-aware and exposes the plugin's metadata.
// FetchSampleData fails with an Unimplemented gRPC status when the
// plugin's Source does not implement DataFetcher; hosts should check
// Meta.SupportsDataPreview before calling it.
type RemoteSource interface {
	GetMeta(ctx context.Context) (*Meta, error)
	Validate(ctx context.Context, config RawConfig) (RawConfig, error)
	Discover(ctx context.Context, config RawConfig) (*DiscoveryResult, error)
	FetchSampleData(ctx context.Context, config RawConfig, a *Asset) ([]string, [][]interface{}, error)
}

// sourcePlugin implements go-plugin's GRPCPlugin for the Source service.
// On the plugin side meta and source are set; on the host side both are
// nil and only GRPCClient is used.
type sourcePlugin struct {
	goplugin.NetRPCUnsupportedPlugin
	meta   Meta
	source Source
}

func (p *sourcePlugin) GRPCServer(broker *goplugin.GRPCBroker, s *grpc.Server) error {
	proto.RegisterSourceServer(s, &grpcServer{meta: p.meta, source: p.source})
	return nil
}

func (p *sourcePlugin) GRPCClient(ctx context.Context, broker *goplugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return &grpcClient{client: proto.NewSourceClient(c)}, nil
}
