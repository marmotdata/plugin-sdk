package pluginsdk

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/marmotdata/plugin-sdk/proto"
)

// grpcServer runs inside the plugin process and adapts the gRPC service
// onto the plugin author's Source implementation.
type grpcServer struct {
	proto.UnimplementedSourceServer
	meta   Meta
	source Source
	// validateMu serialises Validate, which plugins write their config from: a long-running query process takes concurrent calls for several sources, and plugins are written for one call at a time.
	validateMu sync.Mutex
}

// validate runs the plugin's Validate one call at a time.
func (s *grpcServer) validate(config RawConfig) (RawConfig, error) {
	s.validateMu.Lock()
	defer s.validateMu.Unlock()
	return s.source.Validate(config)
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

	validated, err := s.validate(config)
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

	// Plugin authors typically write Validate to parse the raw config
	// and save the result on the Source (s.config = config), and write
	// Discover to read s.config. That works when both methods run on
	// one long-lived object, but that is not how Marmot calls plugins:
	// for every RPC it starts a new plugin process, makes the one
	// call, and kills the process. Validate and Discover therefore run
	// in different processes on different Source instances, and the
	// instance handling Discover has a nil s.config unless Validate
	// runs again here first.
	validated, err := s.validate(config)
	if err != nil {
		return nil, err
	}
	if validated == nil {
		validated = config
	}
	ctx, err = withIdentity(ctx, validated, req.IdentityTokenFile)
	if err != nil {
		return nil, err
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

func (s *grpcServer) FetchSampleData(ctx context.Context, req *proto.FetchSampleDataRequest) (*proto.FetchSampleDataResponse, error) {
	fetcher, ok := s.source.(DataFetcher)
	if !ok {
		return nil, status.Error(codes.Unimplemented, "plugin does not support data preview")
	}

	var config RawConfig
	if err := json.Unmarshal(req.ConfigJson, &config); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	var a Asset
	if err := json.Unmarshal(req.AssetJson, &a); err != nil {
		return nil, fmt.Errorf("unmarshaling asset: %w", err)
	}

	ctx, err := withIdentity(ctx, config, req.IdentityTokenFile)
	if err != nil {
		return nil, err
	}

	columnNames, rows, err := fetcher.FetchSampleData(ctx, config, &a)
	if err != nil {
		return nil, err
	}

	data, err := json.Marshal(SampleData{ColumnNames: columnNames, Rows: rows})
	if err != nil {
		return nil, fmt.Errorf("marshaling sample data: %w", err)
	}
	return &proto.FetchSampleDataResponse{ResultJson: data}, nil
}

func (s *grpcServer) PlanQuery(ctx context.Context, req *proto.PlanQueryRequest) (*proto.PlanQueryResponse, error) {
	querier, ok := s.source.(Querier)
	if !ok {
		return nil, status.Error(codes.Unimplemented, "plugin does not support queries")
	}

	config, queryReq, err := decodeQueryRequest(req.ConfigJson, req.QueryJson)
	if err != nil {
		return nil, err
	}

	// Validate runs first for the same reason it does in Discover: the
	// author's Source may cache parsed config on itself and this RPC can
	// arrive on a fresh process.
	validated, err := s.validate(config)
	if err != nil {
		return nil, err
	}
	if validated == nil {
		validated = config
	}
	ctx, err = withIdentity(ctx, validated, req.IdentityTokenFile)
	if err != nil {
		return nil, err
	}

	plan, err := querier.PlanQuery(ctx, config, queryReq)
	if err != nil {
		return nil, err
	}
	if plan == nil {
		return &proto.PlanQueryResponse{}, nil
	}

	data, err := json.Marshal(plan)
	if err != nil {
		return nil, fmt.Errorf("marshaling query plan: %w", err)
	}
	return &proto.PlanQueryResponse{PlanJson: data}, nil
}

func (s *grpcServer) ExecuteQuery(req *proto.ExecuteQueryRequest, stream proto.Source_ExecuteQueryServer) error {
	querier, ok := s.source.(Querier)
	if !ok {
		return status.Error(codes.Unimplemented, "plugin does not support queries")
	}

	config, queryReq, err := decodeQueryRequest(req.ConfigJson, req.QueryJson)
	if err != nil {
		return err
	}

	validated, err := s.validate(config)
	if err != nil {
		return err
	}
	if validated == nil {
		validated = config
	}
	ctx, err := withIdentity(stream.Context(), validated, req.IdentityTokenFile)
	if err != nil {
		return err
	}

	return querier.ExecuteQuery(ctx, config, queryReq, func(chunk QueryResultChunk) error {
		data, err := json.Marshal(chunk)
		if err != nil {
			return fmt.Errorf("marshaling result chunk: %w", err)
		}
		return stream.Send(&proto.ExecuteQueryChunk{ChunkJson: data})
	})
}

func decodeQueryRequest(configJSON, queryJSON []byte) (RawConfig, QueryRequest, error) {
	var config RawConfig
	if err := json.Unmarshal(configJSON, &config); err != nil {
		return nil, QueryRequest{}, fmt.Errorf("unmarshaling config: %w", err)
	}

	var queryReq QueryRequest
	if err := json.Unmarshal(queryJSON, &queryReq); err != nil {
		return nil, QueryRequest{}, fmt.Errorf("unmarshaling query request: %w", err)
	}
	return config, queryReq, nil
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

	tokenFile, stop, err := identityFor(ctx, config)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.Discover(ctx, &proto.DiscoverRequest{ConfigJson: data, IdentityTokenFile: tokenFile})
	if stopErr := stop(); stopErr != nil && err != nil {
		// A stale token is the likeliest cause of a late failure.
		err = fmt.Errorf("%w (%v)", err, stopErr)
	}
	if err != nil {
		return nil, err
	}

	var result DiscoveryResult
	if err := json.Unmarshal(resp.ResultJson, &result); err != nil {
		return nil, fmt.Errorf("unmarshaling discovery result: %w", err)
	}
	return &result, nil
}

func (c *grpcClient) FetchSampleData(ctx context.Context, config RawConfig, a *Asset) ([]string, [][]any, error) {
	configData, err := json.Marshal(config)
	if err != nil {
		return nil, nil, fmt.Errorf("marshaling config: %w", err)
	}

	assetData, err := json.Marshal(a)
	if err != nil {
		return nil, nil, fmt.Errorf("marshaling asset: %w", err)
	}

	tokenFile, stop, err := identityFor(ctx, config)
	if err != nil {
		return nil, nil, err
	}
	resp, err := c.client.FetchSampleData(ctx, &proto.FetchSampleDataRequest{
		ConfigJson:        configData,
		AssetJson:         assetData,
		IdentityTokenFile: tokenFile,
	})
	if stopErr := stop(); stopErr != nil && err != nil {
		err = fmt.Errorf("%w (%v)", err, stopErr)
	}
	if err != nil {
		return nil, nil, err
	}

	var result SampleData
	if err := json.Unmarshal(resp.ResultJson, &result); err != nil {
		return nil, nil, fmt.Errorf("unmarshaling sample data: %w", err)
	}
	return result.ColumnNames, result.Rows, nil
}

// withIdentity attaches the token file the host minted for this call.
// A federated config that arrived without one is refused here, before
// the plugin builds a credential that would fail less clearly.
func withIdentity(ctx context.Context, config RawConfig, file string) (context.Context, error) {
	if file == "" {
		if Federated(config) {
			return ctx, fmt.Errorf("%w: the host sent none for audience %q", ErrNoIdentityToken, Audience(config))
		}
		return ctx, nil
	}
	return WithIdentityTokenFile(ctx, file), nil
}

func (c *grpcClient) PlanQuery(ctx context.Context, config RawConfig, req QueryRequest) (*QueryPlan, error) {
	configData, queryData, err := encodeQueryRequest(config, req)
	if err != nil {
		return nil, err
	}

	tokenFile, stop, err := identityFor(ctx, config)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.PlanQuery(ctx, &proto.PlanQueryRequest{
		ConfigJson:        configData,
		QueryJson:         queryData,
		IdentityTokenFile: tokenFile,
	})
	if stopErr := stop(); stopErr != nil && err != nil {
		err = fmt.Errorf("%w (%v)", err, stopErr)
	}
	if err != nil {
		return nil, err
	}
	if len(resp.PlanJson) == 0 {
		return nil, nil
	}

	var plan QueryPlan
	if err := json.Unmarshal(resp.PlanJson, &plan); err != nil {
		return nil, fmt.Errorf("unmarshaling query plan: %w", err)
	}
	return &plan, nil
}

func (c *grpcClient) ExecuteQuery(ctx context.Context, config RawConfig, req QueryRequest) (QueryStream, error) {
	configData, queryData, err := encodeQueryRequest(config, req)
	if err != nil {
		return nil, err
	}

	tokenFile, stop, err := identityFor(ctx, config)
	if err != nil {
		return nil, err
	}
	// The stream outlives this call, so it gets its own cancel and keeps
	// the identity token file alive until the caller closes it.
	streamCtx, cancel := context.WithCancel(ctx)
	stream, err := c.client.ExecuteQuery(streamCtx, &proto.ExecuteQueryRequest{
		ConfigJson:        configData,
		QueryJson:         queryData,
		IdentityTokenFile: tokenFile,
	})
	if err != nil {
		cancel()
		if stopErr := stop(); stopErr != nil {
			err = fmt.Errorf("%w (%v)", err, stopErr)
		}
		return nil, err
	}
	return &grpcQueryStream{stream: stream, cancel: cancel, stop: stop}, nil
}

func encodeQueryRequest(config RawConfig, req QueryRequest) ([]byte, []byte, error) {
	configData, err := json.Marshal(config)
	if err != nil {
		return nil, nil, fmt.Errorf("marshaling config: %w", err)
	}

	queryData, err := json.Marshal(req)
	if err != nil {
		return nil, nil, fmt.Errorf("marshaling query request: %w", err)
	}
	return configData, queryData, nil
}

// grpcQueryStream adapts the generated gRPC stream onto QueryStream.
type grpcQueryStream struct {
	stream proto.Source_ExecuteQueryClient
	cancel context.CancelFunc
	stop   func() error
}

func (s *grpcQueryStream) Recv() (*QueryResultChunk, error) {
	msg, err := s.stream.Recv()
	if err != nil {
		return nil, err
	}

	var chunk QueryResultChunk
	if err := json.Unmarshal(msg.ChunkJson, &chunk); err != nil {
		return nil, fmt.Errorf("unmarshaling result chunk: %w", err)
	}
	return &chunk, nil
}

// Close cancels the stream, which also stops the plugin's ExecuteQuery
// when the caller stops reading early, and releases the identity token.
func (s *grpcQueryStream) Close() error {
	s.cancel()
	return s.stop()
}
