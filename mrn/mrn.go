// Package mrn builds and parses Marmot Resource Names (MRNs).
package mrn

import (
	"fmt"
	"strings"
	"unicode"
)

type Format struct {
	Type    string
	Service string
	Name    string
}

// New builds the MRN that identifies an asset. Every part is lowercased
// and cleaned the same way, because an MRN travels through URLs, lineage
// edges and CLI arguments where a space would have to be escaped by every
// caller in turn, and a slash would add a component Parse cannot tell
// apart from the real ones. Both become a dash.
func New(assetType, service, name string) string {
	return fmt.Sprintf("mrn://%s/%s/%s",
		sanitize(assetType),
		sanitize(service),
		sanitize(name))
}

// sanitize replaces slashes and any whitespace, including tabs and
// newlines, with a dash and lowercases what is left.
func sanitize(part string) string {
	return strings.ToLower(strings.Map(func(r rune) rune {
		if r == '/' || unicode.IsSpace(r) {
			return '-'
		}
		return r
	}, part))
}

func Parse(mrn string) (*Format, error) {
	parts := strings.Split(strings.TrimPrefix(mrn, "mrn://"), "/")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid MRN format: expected mrn://<type>/<service>/<name>, got %s", mrn)
	}

	return &Format{
		Type:    parts[0],
		Service: parts[1],
		Name:    parts[2],
	}, nil
}
