package filesource

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestDetectSourceType(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"s3://my-bucket/key", "s3"},
		{"s3://bucket", "s3"},
		{"git::https://github.com/org/repo", "git"},
		{"git::https://github.com/org/repo//subdir?ref=v1", "git"},
		{"/local/path/file.db", "local"},
		{"relative/path", "local"},
		{"", "local"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := DetectSourceType(tt.path)
			if got != tt.want {
				t.Errorf("DetectSourceType(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestParseS3Path(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		wantBucket string
		wantKey    string
		wantErr    bool
	}{
		{
			name:       "bucket and key",
			path:       "s3://my-bucket/path/to/file.db",
			wantBucket: "my-bucket",
			wantKey:    "path/to/file.db",
		},
		{
			name:       "bucket only",
			path:       "s3://my-bucket",
			wantBucket: "my-bucket",
			wantKey:    "",
		},
		{
			name:       "bucket with trailing slash",
			path:       "s3://my-bucket/",
			wantBucket: "my-bucket",
			wantKey:    "",
		},
		{
			name:    "not s3 path",
			path:    "/local/path",
			wantErr: true,
		},
		{
			name:    "empty bucket",
			path:    "s3://",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bucket, key, err := ParseS3Path(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseS3Path(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}
			if bucket != tt.wantBucket {
				t.Errorf("ParseS3Path(%q) bucket = %q, want %q", tt.path, bucket, tt.wantBucket)
			}
			if key != tt.wantKey {
				t.Errorf("ParseS3Path(%q) key = %q, want %q", tt.path, key, tt.wantKey)
			}
		})
	}
}

func TestParseGitPath(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		wantURL     string
		wantSubPath string
		wantRef     string
	}{
		{
			name:    "url only",
			path:    "git::https://github.com/org/repo",
			wantURL: "https://github.com/org/repo",
		},
		{
			name:        "url with subpath",
			path:        "git::https://github.com/org/repo//subdir",
			wantURL:     "https://github.com/org/repo",
			wantSubPath: "subdir",
		},
		{
			name:    "url with ref",
			path:    "git::https://github.com/org/repo?ref=v1.0",
			wantURL: "https://github.com/org/repo",
			wantRef: "v1.0",
		},
		{
			name:        "url with subpath and ref",
			path:        "git::https://github.com/org/repo//docs/api?ref=main",
			wantURL:     "https://github.com/org/repo",
			wantSubPath: "docs/api",
			wantRef:     "main",
		},
		{
			name:    "without prefix",
			path:    "https://github.com/org/repo",
			wantURL: "https://github.com/org/repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url, subPath, ref := ParseGitPath(tt.path)
			if url != tt.wantURL {
				t.Errorf("ParseGitPath(%q) url = %q, want %q", tt.path, url, tt.wantURL)
			}
			if subPath != tt.wantSubPath {
				t.Errorf("ParseGitPath(%q) subPath = %q, want %q", tt.path, subPath, tt.wantSubPath)
			}
			if ref != tt.wantRef {
				t.Errorf("ParseGitPath(%q) ref = %q, want %q", tt.path, ref, tt.wantRef)
			}
		})
	}
}

func TestResolveFilePath_NilConfig_LocalPath(t *testing.T) {
	// Create a temporary file to use as a local path
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.db")
	if err := os.WriteFile(testFile, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}

	localPath, cleanup, err := ResolveFilePath(context.Background(), nil, testFile)
	if err != nil {
		t.Fatalf("ResolveFilePath() error = %v", err)
	}
	defer cleanup()

	if localPath != testFile {
		t.Errorf("ResolveFilePath() localPath = %q, want %q", localPath, testFile)
	}

	// Cleanup should be a no-op (should not panic)
	cleanup()
}

func TestResolveFilePath_LocalSourceType(t *testing.T) {
	path := "/some/local/path"
	fsc := &FileSourceConfig{SourceType: "local"}

	localPath, cleanup, err := ResolveFilePath(context.Background(), fsc, path)
	if err != nil {
		t.Fatalf("ResolveFilePath() error = %v", err)
	}
	defer cleanup()

	if localPath != path {
		t.Errorf("ResolveFilePath() localPath = %q, want %q", localPath, path)
	}
}

func TestResolveFilePath_AutoDetectLocal(t *testing.T) {
	path := "/data/analytics.duckdb"
	fsc := &FileSourceConfig{} // No SourceType set

	localPath, cleanup, err := ResolveFilePath(context.Background(), fsc, path)
	if err != nil {
		t.Fatalf("ResolveFilePath() error = %v", err)
	}
	defer cleanup()

	if localPath != path {
		t.Errorf("ResolveFilePath() localPath = %q, want %q", localPath, path)
	}
}

func TestResolveOptionsListPrefixes(t *testing.T) {
	tests := []struct {
		name    string
		subdirs []string
		base    string
		want    []string
	}{
		{
			name: "no subdirs lists the whole prefix",
			base: "tables/orders/",
			want: []string{"tables/orders/"},
		},
		{
			name:    "single subdir",
			subdirs: []string{"_delta_log"},
			base:    "tables/orders/",
			want:    []string{"tables/orders/_delta_log/"},
		},
		{
			name:    "subdir slashes are normalised",
			subdirs: []string{"/_delta_log/"},
			base:    "tables/orders/",
			want:    []string{"tables/orders/_delta_log/"},
		},
		{
			name:    "nested subdir",
			subdirs: []string{"meta/schema"},
			base:    "tables/orders/",
			want:    []string{"tables/orders/meta/schema/"},
		},
		{
			name:    "multiple subdirs",
			subdirs: []string{"_delta_log", "_change_data"},
			base:    "tables/orders/",
			want:    []string{"tables/orders/_delta_log/", "tables/orders/_change_data/"},
		},
		{
			name:    "empty subdir falls back to the whole prefix",
			subdirs: []string{""},
			base:    "tables/orders/",
			want:    []string{"tables/orders/"},
		},
		{
			name:    "bucket root base",
			subdirs: []string{"_delta_log"},
			base:    "",
			want:    []string{"_delta_log/"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ro resolveOptions
			if tt.subdirs != nil {
				WithSubdirs(tt.subdirs...)(&ro)
			}

			got := ro.listPrefixes(tt.base)
			if len(got) != len(tt.want) {
				t.Fatalf("listPrefixes(%q) = %v, want %v", tt.base, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("listPrefixes(%q)[%d] = %q, want %q", tt.base, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestExtractFileSourceConfig(t *testing.T) {
	rawConfig := map[string]interface{}{
		"source_type": "s3",
		"s3_source": map[string]interface{}{
			"credentials": map[string]interface{}{
				"region": "us-east-1",
			},
		},
	}

	fsc, err := ExtractFileSourceConfig(rawConfig)
	if err != nil {
		t.Fatalf("ExtractFileSourceConfig() error = %v", err)
	}

	if fsc.SourceType != "s3" {
		t.Errorf("SourceType = %q, want %q", fsc.SourceType, "s3")
	}
	if fsc.S3Source == nil {
		t.Fatal("S3Source is nil")
	}
	if fsc.S3Source.Credentials.Region != "us-east-1" {
		t.Errorf("Region = %q, want %q", fsc.S3Source.Credentials.Region, "us-east-1")
	}
}
