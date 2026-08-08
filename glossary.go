package pluginsdk

// GlossaryTerm is a business definition curated in the source system.
// Marmot stores these as first-class terms rather than tags so they can
// be browsed, nested and reused across assets.
type GlossaryTerm struct {
	Name        string `json:"name"`
	Definition  string `json:"definition"`
	Description string `json:"description,omitempty"`
	// Parent is the Name of the term this one sits under, empty for a root.
	Parent   string         `json:"parent,omitempty"`
	Synonyms []string       `json:"synonyms,omitempty"`
	Tags     []string       `json:"tags,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}
