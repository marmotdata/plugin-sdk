package pluginsdk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/marmotdata/plugin-sdk/proto"
)

func TestFederationHelpers(t *testing.T) {
	config := RawConfig{"project_id": "p"}
	assert.False(t, Federated(config))
	assert.Equal(t, "", Audience(config))

	SetAudience(config, "aud")
	assert.True(t, Federated(config))
	assert.Equal(t, "aud", Audience(config))

	SetAudience(config, "")
	assert.False(t, Federated(config))
	_, present := config["audience"]
	assert.False(t, present)
}

func TestTokenFileWritesRefreshesAndCleansUp(t *testing.T) {
	old := identityTokenRefresh
	identityTokenRefresh = 20 * time.Millisecond
	t.Cleanup(func() { identityTokenRefresh = old })

	var mints atomic.Int32
	mint := func(_ context.Context, audience string) (string, error) {
		n := mints.Add(1)
		return fmt.Sprintf("token-%s-%d", audience, n), nil
	}

	f, err := startTokenFile(context.Background(), mint, "aud")
	require.NoError(t, err)

	dirInfo, err := os.Stat(f.dir)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), dirInfo.Mode().Perm(), "directory is private")
	fileInfo, err := os.Stat(f.path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), fileInfo.Mode().Perm(), "token file is private")

	first, err := os.ReadFile(f.path)
	require.NoError(t, err)
	assert.Equal(t, "token-aud-1", string(first))

	require.Eventually(t, func() bool {
		got, err := os.ReadFile(f.path)
		return err == nil && string(got) != "token-aud-1"
	}, time.Second, 5*time.Millisecond, "the file is rewritten with a fresh token")

	require.NoError(t, f.Stop())
	_, err = os.Stat(f.dir)
	assert.True(t, os.IsNotExist(err), "Stop removes the directory")
}

func TestTokenFileKeepsLastTokenWhenRefreshFails(t *testing.T) {
	old := identityTokenRefresh
	identityTokenRefresh = 10 * time.Millisecond
	t.Cleanup(func() { identityTokenRefresh = old })

	var mints atomic.Int32
	mint := func(context.Context, string) (string, error) {
		if mints.Add(1) == 1 {
			return "good", nil
		}
		return "", errors.New("issuer down")
	}

	f, err := startTokenFile(context.Background(), mint, "aud")
	require.NoError(t, err)
	require.Eventually(t, func() bool { return mints.Load() >= 3 }, time.Second, time.Millisecond)

	got, err := os.ReadFile(f.path)
	require.NoError(t, err)
	assert.Equal(t, "good", string(got), "a failed refresh leaves the previous token in place")

	err = f.Stop()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "issuer down")
}

func TestStartTokenFileFailsWhenFirstMintFails(t *testing.T) {
	_, err := startTokenFile(context.Background(), func(context.Context, string) (string, error) {
		return "", errors.New("no issuer")
	}, "aud")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no issuer")
}

// fakeSourceClient stands in for the plugin process on the host side and
// records what the Discover request carried.
type fakeSourceClient struct {
	proto.SourceClient
	discover func(*proto.DiscoverRequest) (*proto.DiscoverResponse, error)
	sample   func(*proto.FetchSampleDataRequest) (*proto.FetchSampleDataResponse, error)
}

func (f *fakeSourceClient) Discover(_ context.Context, in *proto.DiscoverRequest, _ ...grpc.CallOption) (*proto.DiscoverResponse, error) {
	return f.discover(in)
}

func (f *fakeSourceClient) FetchSampleData(_ context.Context, in *proto.FetchSampleDataRequest, _ ...grpc.CallOption) (*proto.FetchSampleDataResponse, error) {
	return f.sample(in)
}

func emptyDiscovery(t *testing.T) *proto.DiscoverResponse {
	data, err := json.Marshal(DiscoveryResult{})
	require.NoError(t, err)
	return &proto.DiscoverResponse{ResultJson: data}
}

func TestClientDiscoverPassesNoFileForPlainConfig(t *testing.T) {
	var got *proto.DiscoverRequest
	c := &grpcClient{client: &fakeSourceClient{discover: func(in *proto.DiscoverRequest) (*proto.DiscoverResponse, error) {
		got = in
		return emptyDiscovery(t), nil
	}}}

	_, err := c.Discover(context.Background(), RawConfig{"host": "db"})
	require.NoError(t, err)
	assert.Equal(t, "", got.IdentityTokenFile)
}

func TestClientDiscoverRefusesFederatedConfigWithoutMinter(t *testing.T) {
	called := false
	c := &grpcClient{client: &fakeSourceClient{discover: func(*proto.DiscoverRequest) (*proto.DiscoverResponse, error) {
		called = true
		return emptyDiscovery(t), nil
	}}}

	_, err := c.Discover(context.Background(), RawConfig{"audience": "sts.amazonaws.com"})
	require.ErrorIs(t, err, ErrNoIdentityMinter)
	assert.Contains(t, err.Error(), "sts.amazonaws.com")
	assert.False(t, called, "the plugin is never started")
}

func TestClientDiscoverMintsWritesAndRemovesTheToken(t *testing.T) {
	var seenPath, seenToken string
	c := &grpcClient{client: &fakeSourceClient{discover: func(in *proto.DiscoverRequest) (*proto.DiscoverResponse, error) {
		seenPath = in.IdentityTokenFile
		data, err := os.ReadFile(in.IdentityTokenFile)
		if err != nil {
			return nil, err
		}
		seenToken = string(data)
		return emptyDiscovery(t), nil
	}}}

	var audience string
	ctx := WithIdentityMinter(context.Background(), func(_ context.Context, aud string) (string, error) {
		audience = aud
		return "jwt-for-" + aud, nil
	})

	_, err := c.Discover(ctx, RawConfig{"audience": "https://iam.googleapis.com/projects/1/locations/global/workloadIdentityPools/p/providers/x"})
	require.NoError(t, err)
	assert.Equal(t, "https://iam.googleapis.com/projects/1/locations/global/workloadIdentityPools/p/providers/x", audience)
	assert.Equal(t, "jwt-for-"+audience, seenToken, "the plugin reads the minted token from the file")
	assert.Equal(t, "token", filepath.Base(seenPath))
	_, statErr := os.Stat(seenPath)
	assert.True(t, os.IsNotExist(statErr), "the file is gone once the call returns")
}

func TestClientFetchSampleDataCarriesTheToken(t *testing.T) {
	var seenToken string
	c := &grpcClient{client: &fakeSourceClient{sample: func(in *proto.FetchSampleDataRequest) (*proto.FetchSampleDataResponse, error) {
		data, err := os.ReadFile(in.IdentityTokenFile)
		if err != nil {
			return nil, err
		}
		seenToken = string(data)
		out, _ := json.Marshal(SampleData{})
		return &proto.FetchSampleDataResponse{ResultJson: out}, nil
	}}}
	ctx := WithIdentityMinter(context.Background(), func(context.Context, string) (string, error) { return "jwt", nil })

	_, _, err := c.FetchSampleData(ctx, RawConfig{"audience": "api://AzureADTokenExchange"}, &Asset{})
	require.NoError(t, err)
	assert.Equal(t, "jwt", seenToken)
}

// captureSource records the context Discover and FetchSampleData receive.
type captureSource struct {
	ctx      context.Context
	validate func(RawConfig) (RawConfig, error)
}

func (s *captureSource) Validate(config RawConfig) (RawConfig, error) {
	if s.validate != nil {
		return s.validate(config)
	}
	return config, nil
}

func (s *captureSource) Discover(ctx context.Context, _ RawConfig) (*DiscoveryResult, error) {
	s.ctx = ctx
	return &DiscoveryResult{}, nil
}

func (s *captureSource) FetchSampleData(ctx context.Context, _ RawConfig, _ *Asset) ([]string, [][]any, error) {
	s.ctx = ctx
	return nil, nil, nil
}

func TestServerDiscoverExposesTheTokenFile(t *testing.T) {
	src := &captureSource{}
	srv := &grpcServer{source: src}
	config, _ := json.Marshal(RawConfig{"audience": "aud"})

	_, err := srv.Discover(context.Background(), &proto.DiscoverRequest{ConfigJson: config, IdentityTokenFile: "/run/marmot/token"})
	require.NoError(t, err)
	path, ok := IdentityTokenFile(src.ctx)
	assert.True(t, ok)
	assert.Equal(t, "/run/marmot/token", path)
}

func TestServerDiscoverRefusesFederatedConfigWithoutTokenFile(t *testing.T) {
	// The audience is derived by Validate, as a real plugin does; the
	// request config carries none yet.
	src := &captureSource{validate: func(c RawConfig) (RawConfig, error) {
		SetAudience(c, "sts.amazonaws.com")
		return c, nil
	}}
	srv := &grpcServer{source: src}
	config, _ := json.Marshal(RawConfig{"role_arn": "arn:aws:iam::123456789012:role/r"})

	_, err := srv.Discover(context.Background(), &proto.DiscoverRequest{ConfigJson: config})
	require.ErrorIs(t, err, ErrNoIdentityToken)
	assert.Nil(t, src.ctx, "Discover never runs")
}

func TestServerDiscoverPlainConfigHasNoTokenFile(t *testing.T) {
	src := &captureSource{}
	srv := &grpcServer{source: src}
	config, _ := json.Marshal(RawConfig{"host": "db"})

	_, err := srv.Discover(context.Background(), &proto.DiscoverRequest{ConfigJson: config})
	require.NoError(t, err)
	_, ok := IdentityTokenFile(src.ctx)
	assert.False(t, ok)
}

func TestServerFetchSampleDataExposesTheTokenFile(t *testing.T) {
	src := &captureSource{}
	srv := &grpcServer{source: src}
	config, _ := json.Marshal(RawConfig{"audience": "aud"})
	asset, _ := json.Marshal(Asset{})

	_, err := srv.FetchSampleData(context.Background(), &proto.FetchSampleDataRequest{ConfigJson: config, AssetJson: asset, IdentityTokenFile: "/run/marmot/token"})
	require.NoError(t, err)
	path, ok := IdentityTokenFile(src.ctx)
	assert.True(t, ok)
	assert.Equal(t, "/run/marmot/token", path)
}
