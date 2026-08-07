package pluginsdk

// SampleData is the result of a FetchSampleData call: column names and
// the sampled rows.
type SampleData struct {
	ColumnNames []string `json:"column_names"`
	Rows        [][]any  `json:"rows"`
}
