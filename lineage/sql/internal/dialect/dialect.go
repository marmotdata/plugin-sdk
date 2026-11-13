// Package dialect is the registry the lineage/sql dialect subpackages add
// themselves to, so a plugin only links the parsers it imports.
package dialect

import (
	"strings"
	"sync"

	storepb "github.com/bytebase/bytebase/backend/generated-go/store"
)

// Dialect is how the extractor drives one Bytebase engine.
type Dialect struct {
	Engine storepb.Engine
	// DefaultSchema fills ExtractRequest.DefaultSchema when it is empty: the
	// schema an unqualified table resolves to. Empty for engines without a
	// schema level, such as MySQL.
	DefaultSchema string
	// SchemaIsDatabase is set for engines whose parser has one level where
	// plugins have two: the source's Schema is used as the database and its
	// Database is ignored. BigQuery's parser knows datasets but not projects.
	SchemaIsDatabase bool
	// RewriteSQL, when set, adapts the SQL before parsing, for name forms the
	// parser cannot resolve.
	RewriteSQL func(string) string
	// Fallback names a dialect to retry with when this one yields no edges
	// and no error: Redshift's parser drops a whole query on any type it
	// cannot check, where PostgreSQL's reads the same SQL leniently.
	Fallback string
	// ReservedDatabases are names the parser silently ignores (Bytebase's
	// Oracle extractor skips its list of system and sample schemas, which
	// includes HR and SCOTT). Sources in them are presented to the parser
	// under a placeholder name.
	ReservedDatabases []string
}

var (
	mu       sync.RWMutex
	dialects = map[string]Dialect{}
)

// Register adds d under each name, case-insensitively.
func Register(d Dialect, names ...string) {
	mu.Lock()
	defer mu.Unlock()
	for _, n := range names {
		dialects[strings.ToLower(n)] = d
	}
}

// Lookup returns the dialect registered under name.
func Lookup(name string) (Dialect, bool) {
	mu.RLock()
	defer mu.RUnlock()
	d, ok := dialects[strings.ToLower(name)]
	return d, ok
}
