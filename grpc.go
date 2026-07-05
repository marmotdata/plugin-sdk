package pluginsdk

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/marmotdata/plugin-sdk/proto"
)

// grpcServer runs inside the plugin process and adapts the gRPC service
// onto the plugin author's Source implementation.
type grpcServer struct {
	proto.UnimplementedSourceServer
	meta   Meta
	source Source
}

func (s *grpcServer) GetMeta(ctx context.Context, req *proto.GetMetaRequest) (*proto.GetMetaResponse, error) {
	data, err := json.Marshal(s.meta)
	if err != nil {
		return nil, fmt.Errorf("marshaling plugin meta: %w", err)
	}
	return &proto.GetMetaResponse{MetaJson: data}, nil
}

func (s *grpcServer) Validate(ctx context.Context, req *proto.ValidateRequest) (*proto.ValidateResponse, error) {
	var config RawConfig
	if err := json.Unmarshal(req.ConfigJson, &config); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	validated, err := s.source.Validate(config)
	if err != nil {
		return nil, err
	}

	data, err := json.Marshal(validated)
	if err != nil {
		return nil, fmt.Errorf("marshaling validated config: %w", err)
	}
	return &proto.ValidateResponse{ConfigJson: data}, nil
}

func (s *grpcServer) Discover(ctx context.Context, req *proto.DiscoverRequest) (*proto.DiscoverResponse, error) {
	var config RawConfig
	if err := json.Unmarshal(req.ConfigJson, &config); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	result, err := s.source.Discover(ctx, config)
	if err != nil {
		return nil, err
	}

	data, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("marshaling discovery result: %w", err)
	}
	return &proto.DiscoverResponse{ResultJson: data}, nil
}

// grpcClient runs inside the host process and adapts RemoteSource calls
// onto the plugin's gRPC service.
type grpcClient struct {
	client proto.SourceClient
}

func (c *grpcClient) GetMeta(ctx context.Context) (*Meta, error) {
	resp, err := c.client.GetMeta(ctx, &proto.GetMetaRequest{})
	if err != nil {
		return nil, err
	}

	var meta Meta
	if err := json.Unmarshal(resp.MetaJson, &meta); err != nil {
		return nil, fmt.Errorf("unmarshaling plugin meta: %w", err)
	}
	return &meta, nil
}

func (c *grpcClient) Validate(ctx context.Context, config RawConfig) (RawConfig, error) {
	data, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("marshaling config: %w", err)
	}

	resp, err := c.client.Validate(ctx, &proto.ValidateRequest{ConfigJson: data})
	if err != nil {
		return nil, err
	}

	var validated RawConfig
	if err := json.Unmarshal(resp.ConfigJson, &validated); err != nil {
		return nil, fmt.Errorf("unmarshaling validated config: %w", err)
	}
	return validated, nil
}

func (c *grpcClient) Discover(ctx context.Context, config RawConfig) (*DiscoveryResult, error) {
	data, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("marshaling config: %w", err)
	}

	resp, err := c.client.Discover(ctx, &proto.DiscoverRequest{ConfigJson: data})
	if err != nil {
		return nil, err
	}

	var result DiscoveryResult
	if err := json.Unmarshal(resp.ResultJson, &result); err != nil {
		return nil, fmt.Errorf("unmarshaling discovery result: %w", err)
	}
	return &result, nil
}
