package pluginsdk

// Documentation is markdown documentation attached to an asset.
type Documentation struct {
	MRN     string `json:"mrn"`
	Content string `json:"content"`
	Source  string `json:"source"`
}
