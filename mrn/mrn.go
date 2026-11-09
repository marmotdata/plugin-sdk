// Package mrn builds and parses Marmot Resource Names (MRNs).
//
// An MRN names one thing in a Marmot catalog and starts with the kind of
// thing it names:
//
//	mrn://asset/<type>/<provider>[@<zone>]/<name>
//	mrn://glossary-term/<source>/<path>
//	mrn://data-product/<slug>
//	mrn://team/<slug>
//	mrn://user/<username>
//	mrn://pipeline/<name>
//	mrn://source/<name>
//
// An asset's name is the full path to the object inside the system that
// holds it, for example analytics.public.orders for a PostgreSQL table, so
// the same table name in two schemas or two databases never collides. The
// optional zone tells apart two systems of the same technology, such as
// two PostgreSQL servers holding identical databases.
//
// MRNs from before kinds existed, mrn://<type>/<provider>/<name>, are still
// accepted everywhere: Parse reads them as assets, and Marmot stores and
// returns the kinded form.
package mrn

import (
	"errors"
	"fmt"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Prefix starts every MRN.
const Prefix = "mrn://"

// Kind is what an MRN names.
type Kind string

const (
	KindAsset        Kind = "asset"
	KindGlossaryTerm Kind = "glossary-term"
	KindDataProduct  Kind = "data-product"
	KindTeam         Kind = "team"
	KindUser         Kind = "user"
	KindPipeline     Kind = "pipeline"
	KindSource       Kind = "source"
)

// kindParts is how many path segments follow the kind for each kind. An
// asset has type, provider and name; a glossary term has source and path;
// the rest have a single name.
var kindParts = map[Kind]int{
	KindAsset:        3,
	KindGlossaryTerm: 2,
	KindDataProduct:  1,
	KindTeam:         1,
	KindUser:         1,
	KindPipeline:     1,
	KindSource:       1,
}

// Kinds returns every kind Marmot knows, which are also the words an asset
// type cannot be mistaken for in a legacy MRN.
func Kinds() []Kind {
	return []Kind{KindAsset, KindGlossaryTerm, KindDataProduct, KindTeam, KindUser, KindPipeline, KindSource}
}

// IsKind reports whether s names a known kind.
func IsKind(s string) bool {
	_, ok := kindParts[Kind(s)]
	return ok
}

// ErrInvalid is wrapped by every error Parse returns.
var ErrInvalid = errors.New("invalid MRN")

// MRN is a parsed Marmot Resource Name. Build one with Asset, GlossaryTerm
// and the other constructors rather than by hand, so every part is cleaned
// the way Parse expects.
type MRN struct {
	Kind Kind
	// Type and Provider are set for assets only.
	Type     string
	Provider string
	// Zone tells apart two systems of the same provider. Empty means the
	// provider's no zone. Set for assets only.
	Zone string
	// Source is the glossary a term came from, "marmot" for terms made in
	// Marmot. Set for glossary terms only.
	Source string
	// Name is the last segment: the asset's qualified name, the term's path,
	// or the slug of any other kind.
	Name string
	// Legacy is true when the MRN was parsed from the form without a kind.
	Legacy bool
}

// Asset builds an asset MRN. Each part of the path is one level of the
// system's own hierarchy, outermost first, for example
// Asset("Table", "PostgreSQL", "analytics", "public", "orders"). A single
// part is used as it is, dots included, so a Kafka topic named orders.v1
// keeps its name. When there are several parts, a part holding a dot is
// quoted so that "my.db".public.orders stays three parts.
func Asset(assetType, provider string, path ...string) MRN {
	return MRN{
		Kind:     KindAsset,
		Type:     sanitize(assetType),
		Provider: sanitizeSegment(provider),
		Name:     JoinPath(path...),
	}
}

// GlossaryTerm builds a glossary term MRN. Source is where the glossary came
// from, "marmot" for terms made in Marmot, and path runs from the top-level
// term down to this one.
func GlossaryTerm(source string, path ...string) MRN {
	return MRN{Kind: KindGlossaryTerm, Source: sanitizeSegment(source), Name: JoinPath(path...)}
}

// DataProduct builds a data product MRN from the product's name.
func DataProduct(name string) MRN { return MRN{Kind: KindDataProduct, Name: Slug(name)} }

// Team builds a team MRN from the team's name.
func Team(name string) MRN { return MRN{Kind: KindTeam, Name: Slug(name)} }

// User builds a user MRN from the username.
func User(username string) MRN { return MRN{Kind: KindUser, Name: sanitize(username)} }

// Pipeline builds a pipeline MRN from the pipeline's name.
func Pipeline(name string) MRN { return MRN{Kind: KindPipeline, Name: sanitize(name)} }

// Source builds the MRN of a queryable source.
func Source(name string) MRN { return MRN{Kind: KindSource, Name: sanitize(name)} }

// WithZone returns the MRN with its zone set. An empty zone
// means no zone.
func (m MRN) WithZone(zone string) MRN {
	m.Zone = sanitizeSegment(zone)
	return m
}

// WithHost returns the MRN with a host hint in place of a zone. A
// plugin uses it for an asset another plugin owns when all it knows is the
// address it was given, for example a Kafka Connect sink's JDBC URL. Marmot
// resolves the hint to the zone that host belongs to and never stores
// it; with no match no zone is used.
func (m MRN) WithHost(host string) MRN {
	host = strings.TrimSpace(host)
	if host == "" {
		m.Zone = ""
		return m
	}
	m.Zone = HostHintPrefix + sanitizeSegment(host)
	return m
}

// HostHintPrefix marks a zone that is a host hint rather than a name.
const HostHintPrefix = "host="

// HostHint returns the host an MRN's zone hints at, if it is a hint.
func (m MRN) HostHint() (string, bool) {
	if strings.HasPrefix(m.Zone, HostHintPrefix) {
		return strings.TrimPrefix(m.Zone, HostHintPrefix), true
	}
	return "", false
}

// String returns the MRN in its kinded form. A legacy MRN is written in the
// kinded form too: it names the same asset.
func (m MRN) String() string {
	switch m.Kind {
	case KindAsset, "":
		provider := m.Provider
		if m.Zone != "" {
			provider += "@" + m.Zone
		}
		return Prefix + string(KindAsset) + "/" + m.Type + "/" + provider + "/" + m.Name
	case KindGlossaryTerm:
		return Prefix + string(m.Kind) + "/" + m.Source + "/" + m.Name
	default:
		return Prefix + string(m.Kind) + "/" + m.Name
	}
}

// LegacyString returns an asset MRN in the form without a kind, which is what a
// Marmot release before kinds stored for the same asset. The zone is
// dropped because that form had none. It is empty for other kinds.
func (m MRN) LegacyString() string {
	if m.Kind != KindAsset {
		return ""
	}
	return Prefix + m.Type + "/" + m.Provider + "/" + m.Name
}

// IsZero reports whether m is the zero MRN.
func (m MRN) IsZero() bool { return m == MRN{} }

// Equal reports whether two MRNs name the same thing.
func (m MRN) Equal(o MRN) bool { return m.String() == o.String() }

// Path splits the name into the parts Asset or GlossaryTerm joined. A name
// written by hand without quotes is split at every dot, which can split a
// single-part name such as a Kafka topic. Identity never depends on Path:
// two MRNs are the same exactly when their strings are.
func (m MRN) Path() []string { return SplitPath(m.Name) }

// ProviderSegment is the provider with its zone, as it appears in the
// MRN and in Marmot's asset URLs.
func (m MRN) ProviderSegment() string {
	if m.Zone == "" {
		return m.Provider
	}
	return m.Provider + "@" + m.Zone
}

// MarshalText writes the MRN as its string, so an MRN field encodes as a
// plain JSON or YAML string.
func (m MRN) MarshalText() ([]byte, error) {
	if m.IsZero() {
		return []byte{}, nil
	}
	return []byte(m.String()), nil
}

// UnmarshalText parses an MRN string, legacy or kinded.
func (m *MRN) UnmarshalText(b []byte) error {
	if len(b) == 0 {
		*m = MRN{}
		return nil
	}
	parsed, err := ParseMRN(string(b))
	if err != nil {
		return err
	}
	*m = parsed
	return nil
}

// ParseMRN parses an MRN in either form and cleans every part the way the
// constructors do, so ParseMRN(s).String() is the one canonical spelling of
// whatever s names.
//
// A string whose first segment is a kind is read as that kind. Anything else
// with three segments is a legacy asset MRN. The one ambiguous case is a
// legacy MRN whose asset type is spelled like a kind with the same number of
// segments, mrn://glossary-term/<x>/<y>; it is read as the kind.
func ParseMRN(s string) (MRN, error) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(strings.ToLower(s), Prefix) {
		return MRN{}, fmt.Errorf("%w: %q does not start with %s", ErrInvalid, s, Prefix)
	}
	rest := s[len(Prefix):]
	parts := strings.Split(rest, "/")
	for _, p := range parts {
		if p == "" {
			return MRN{}, fmt.Errorf("%w: %q has an empty segment", ErrInvalid, s)
		}
	}

	kind := Kind(strings.ToLower(parts[0]))
	if want, ok := kindParts[kind]; ok && len(parts)-1 == want {
		switch kind {
		case KindAsset:
			return parseAsset(parts[1], parts[2], parts[3], false), nil
		case KindGlossaryTerm:
			return MRN{Kind: kind, Source: sanitizeSegment(parts[1]), Name: sanitizeName(parts[2])}, nil
		default:
			return MRN{Kind: kind, Name: sanitizeName(parts[1])}, nil
		}
	}

	if len(parts) == 3 {
		return parseAsset(parts[0], parts[1], parts[2], true), nil
	}

	return MRN{}, fmt.Errorf("%w: %q is neither mrn://<kind>/... nor mrn://<type>/<provider>/<name>", ErrInvalid, s)
}

func parseAsset(assetType, providerSegment, name string, legacy bool) MRN {
	provider, zone := providerSegment, ""
	if i := strings.Index(providerSegment, "@"); i >= 0 {
		provider, zone = providerSegment[:i], providerSegment[i+1:]
	}
	m := MRN{
		Kind:     KindAsset,
		Type:     sanitize(assetType),
		Provider: sanitizeSegment(provider),
		Name:     sanitizeName(name),
		Legacy:   legacy,
	}
	if strings.HasPrefix(strings.ToLower(zone), HostHintPrefix) {
		m.Zone = HostHintPrefix + sanitizeSegment(zone[len(HostHintPrefix):])
	} else {
		m.Zone = sanitizeSegment(zone)
	}
	return m
}

// Canonical returns the canonical spelling of an MRN in either form.
func Canonical(s string) (string, error) {
	m, err := ParseMRN(s)
	if err != nil {
		return "", err
	}
	return m.String(), nil
}

// MustCanonical is Canonical for MRNs known to be well formed, such as ones
// built by this package. It returns s unchanged when s does not parse.
func MustCanonical(s string) string {
	c, err := Canonical(s)
	if err != nil {
		return s
	}
	return c
}

// JoinPath joins the levels of a qualified name with dots. A single part is
// returned as it is; with several, a part holding a dot or a quote is
// wrapped in double quotes with any quote inside doubled.
func JoinPath(parts ...string) string {
	cleaned := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = sanitizeName(p); p != "" {
			cleaned = append(cleaned, p)
		}
	}
	if len(cleaned) == 1 {
		return cleaned[0]
	}
	for i, p := range cleaned {
		if strings.ContainsAny(p, `."`) {
			cleaned[i] = `"` + strings.ReplaceAll(p, `"`, `""`) + `"`
		}
	}
	return strings.Join(cleaned, ".")
}

// SplitPath undoes JoinPath: it splits at dots outside double quotes and
// unquotes each part.
func SplitPath(name string) []string {
	var parts []string
	var cur strings.Builder
	quoted := false
	for i := 0; i < len(name); i++ {
		c := name[i]
		switch {
		case c == '"' && quoted && i+1 < len(name) && name[i+1] == '"':
			cur.WriteByte('"')
			i++
		case c == '"':
			quoted = !quoted
		case c == '.' && !quoted:
			parts = append(parts, cur.String())
			cur.Reset()
		default:
			cur.WriteByte(c)
		}
	}
	return append(parts, cur.String())
}

// Slug turns a display name into an MRN segment: lowercase, with every run
// of characters other than letters, digits, dots and underscores replaced
// by a single dash.
func Slug(name string) string {
	name = norm.NFC.String(strings.ToLower(strings.TrimSpace(name)))
	var b strings.Builder
	dash := false
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '_' {
			b.WriteRune(r)
			dash = false
			continue
		}
		if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	return strings.TrimSuffix(b.String(), "-")
}

// sanitize lowercases a part, normalises it to NFC so the same name typed
// two ways is one MRN, and replaces slashes and whitespace, which would
// break the segment structure, with a dash.
func sanitize(part string) string {
	return strings.ToLower(strings.Map(func(r rune) rune {
		if r == '/' || unicode.IsSpace(r) {
			return '-'
		}
		return r
	}, norm.NFC.String(part)))
}

// sanitizeSegment is sanitize for a provider, zone or source, where an
// @ would be read as the start of a zone.
func sanitizeSegment(part string) string {
	return strings.ReplaceAll(sanitize(part), "@", "-")
}

// sanitizeName is sanitize for the last segment, which may hold anything
// but a slash or whitespace.
func sanitizeName(part string) string { return sanitize(part) }

// New builds an asset MRN from a single, already qualified name, for
// example New("Table", "PostgreSQL", "analytics.public.orders").
//
// Prefer Asset, which takes the name's levels separately and quotes any
// level holding a dot, and BaseConfig.AssetMRN, which also carries the
// pipeline's zone.
func New(assetType, service, name string) string {
	return MRN{Kind: KindAsset, Type: sanitize(assetType), Provider: sanitizeSegment(service), Name: sanitizeName(name)}.String()
}
