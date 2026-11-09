package pluginsdk

// DiscoveryResult is the outcome of one discovery run.
type DiscoveryResult struct {
	Assets        []Asset           `json:"assets"`
	Lineage       []LineageEdge     `json:"lineage"`
	Documentation []Documentation   `json:"documentation"`
	Statistics    []Statistic       `json:"statistics"`
	RunHistory    []AssetRunHistory `json:"run_history,omitempty"`
	GlossaryTerms []GlossaryTerm    `json:"glossary_terms,omitempty"`
	// ZoneBindings tell Marmot which zone the hosts this plugin
	// connected to belong to, so another plugin that only knows a host, such
	// as a Kafka Connect sink, lands its lineage on the right system.
	ZoneBindings []ZoneBinding `json:"zone_bindings,omitempty"`
}

// ZoneBinding says that a provider reached at Host is Zone. An
// empty Zone is no zone.
type ZoneBinding struct {
	Provider string `json:"provider"`
	Host     string `json:"host"`
	Zone string `json:"zone,omitempty"`
}
