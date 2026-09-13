package pluginsdk

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Federation is embedded inline in a plugin's config. A plugin whose
// credentials exchange a Marmot identity token for a cloud credential
// sets Audience in its Validate, derived from its own settings: the
// Workload Identity provider on Google Cloud, sts.amazonaws.com on AWS,
// api://AzureADTokenExchange on Azure. The host then mints a token for
// that audience on every Discover and FetchSampleData and hands it to
// the plugin process as a file it keeps fresh; see IdentityTokenFile.
// No audience means the plugin runs on whatever credentials its other
// fields name.
//
//	type Config struct {
//	    pluginsdk.BaseConfig `json:",inline"`
//	    pluginsdk.Federation `json:",inline"`
//	    pluginsdk.GCPConfig  `json:",inline"`
//	}
type Federation struct {
	Audience string `json:"audience,omitempty" label:"Audience" description:"Audience of the Marmot identity token. Derived from the credentials' federation settings; set it only when the cloud side expects another"`
}

// Federated reports whether a config asks for identity tokens. It reads
// the top-level audience of the raw config, so the host enforces the
// federated contract without knowing the plugin.
func Federated(config RawConfig) bool {
	return Audience(config) != ""
}

// Audience returns the audience of a raw config, or "".
func Audience(config RawConfig) string {
	audience, _ := config["audience"].(string)
	return audience
}

// SetAudience writes a derived audience into the raw config a Validate
// returns, so the host stores it and mints for it. An empty audience
// removes the key.
func SetAudience(config RawConfig, audience string) {
	if audience == "" {
		delete(config, "audience")
		return
	}
	config["audience"] = audience
}

// IdentityMinter mints a Marmot identity token for an audience. The
// host installs one with WithIdentityMinter; the SDK calls it before a
// federated Discover or FetchSampleData and again while the call runs.
type IdentityMinter func(ctx context.Context, audience string) (string, error)

var (
	// ErrNoIdentityMinter is returned to the host when a config federates
	// but the context carries no minter: open-source Marmot and `marmot
	// ingest` cannot mint pipeline identities.
	ErrNoIdentityMinter = errors.New("this configuration federates but this Marmot host cannot mint workload identity tokens; federated plugin credentials need Marmot Cloud or Marmot Enterprise with a workload identity issuer")
	// ErrNoIdentityToken is returned inside the plugin when federated
	// credentials are built on a call that carries no token file. The
	// SDK refuses such calls before the plugin sees them; this covers
	// direct use of the credential helpers.
	ErrNoIdentityToken = errors.New("federated auth: this call carries no Marmot identity token")
)

type identityMinterKey struct{}

// WithIdentityMinter attaches the host's minter to a context passed to
// RemoteSource.Discover or FetchSampleData.
func WithIdentityMinter(ctx context.Context, mint IdentityMinter) context.Context {
	return context.WithValue(ctx, identityMinterKey{}, mint)
}

func identityMinter(ctx context.Context) (IdentityMinter, bool) {
	mint, ok := ctx.Value(identityMinterKey{}).(IdentityMinter)
	return mint, ok && mint != nil
}

type identityTokenFileKey struct{}

// WithIdentityTokenFile attaches the path of the token file the host
// keeps fresh for the current call. The SDK server sets it from the
// request; plugins read it with IdentityTokenFile.
func WithIdentityTokenFile(ctx context.Context, path string) context.Context {
	return context.WithValue(ctx, identityTokenFileKey{}, path)
}

// IdentityTokenFile returns the path of the file holding the Marmot
// identity token minted for the current call, or false when the host
// minted none. The file is rewritten while the call runs, so read it
// each time a subject token is needed, as the cloud SDKs' file-based
// credential sources do.
func IdentityTokenFile(ctx context.Context) (string, bool) {
	path, ok := ctx.Value(identityTokenFileKey{}).(string)
	return path, ok && path != ""
}

// identityTokenRefresh is how often the host rewrites the token file.
// Marmot tokens live five minutes; two keeps a fresh one on disk with
// room for a slow mint. A variable so tests can shorten it.
var identityTokenRefresh = 2 * time.Minute

// tokenFile is a token the host keeps fresh on disk for one plugin call:
// a private directory, one file, rewritten by atomic rename until Stop
// removes it all.
type tokenFile struct {
	dir  string
	path string

	cancel context.CancelFunc
	done   chan struct{}

	mu      sync.Mutex
	lastErr error
}

// startTokenFile mints the first token, writes it, and starts the
// refresh loop. It fails only if the first mint or write fails; later
// failures leave the previous token in place and surface from Stop.
func startTokenFile(ctx context.Context, mint IdentityMinter, audience string) (*tokenFile, error) {
	token, err := mint(ctx, audience)
	if err != nil {
		return nil, fmt.Errorf("minting workload identity token: %w", err)
	}
	dir, err := os.MkdirTemp("", "marmot-identity-")
	if err != nil {
		return nil, fmt.Errorf("creating identity token directory: %w", err)
	}
	f := &tokenFile{dir: dir, path: filepath.Join(dir, "token"), done: make(chan struct{})}
	if err := f.write(token); err != nil {
		_ = os.RemoveAll(dir)
		return nil, err
	}

	loopCtx, cancel := context.WithCancel(ctx)
	f.cancel = cancel
	go f.refresh(loopCtx, mint, audience)
	return f, nil
}

func (f *tokenFile) refresh(ctx context.Context, mint IdentityMinter, audience string) {
	defer close(f.done)
	ticker := time.NewTicker(identityTokenRefresh)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		token, err := mint(ctx, audience)
		if err == nil {
			err = f.write(token)
		}
		if err != nil && ctx.Err() == nil {
			f.mu.Lock()
			f.lastErr = fmt.Errorf("refreshing workload identity token: %w", err)
			f.mu.Unlock()
		}
	}
}

// write replaces the token atomically: a plugin reading during a refresh
// sees the old token or the new one, never a partial file.
func (f *tokenFile) write(token string) error {
	tmp := f.path + ".tmp"
	if err := os.WriteFile(tmp, []byte(token), 0o600); err != nil {
		return fmt.Errorf("writing identity token: %w", err)
	}
	if err := os.Rename(tmp, f.path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("writing identity token: %w", err)
	}
	return nil
}

// Stop ends the refresh loop and removes the token. It returns the
// last refresh error, if any, for the host to log; the call itself has
// already succeeded or failed on its own terms.
func (f *tokenFile) Stop() error {
	f.cancel()
	<-f.done
	_ = os.RemoveAll(f.dir)
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.lastErr
}

// identityFor prepares the token file for a call on config. A config
// that does not federate needs nothing; one that does needs a minter on
// the context. The returned stop is always safe to call.
func identityFor(ctx context.Context, config RawConfig) (path string, stop func() error, err error) {
	noop := func() error { return nil }
	audience := Audience(config)
	if audience == "" {
		return "", noop, nil
	}
	mint, ok := identityMinter(ctx)
	if !ok {
		return "", noop, fmt.Errorf("%w (audience %q)", ErrNoIdentityMinter, audience)
	}
	f, err := startTokenFile(ctx, mint, audience)
	if err != nil {
		return "", noop, err
	}
	return f.path, f.Stop, nil
}

// validationError is a single-field ValidationErrors, for the credential
// helpers' own checks.
func validationError(field, message string) error {
	return ValidationErrors{Errors: []ValidationError{{Field: field, Message: message}}}
}
