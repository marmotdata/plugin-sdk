package pluginsdk

// DiscoveryResult is the outcome of one discovery run.
type DiscoveryResult struct {
	Assets        []Asset           `json:"assets"`
	Lineage       []LineageEdge     `json:"lineage"`
	Documentation []Documentation   `json:"documentation"`
	Statistics    []Statistic       `json:"statistics"`
	RunHistory    []AssetRunHistory `json:"run_history,omitempty"`
	GlossaryTerms []GlossaryTerm    `json:"glossary_terms,omitempty"`
}
