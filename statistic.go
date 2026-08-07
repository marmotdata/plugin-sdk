package pluginsdk

// Statistic is a single metric value for an asset.
type Statistic struct {
	AssetMRN   string  `json:"asset_mrn"`
	MetricName string  `json:"metric_name"`
	Value      float64 `json:"value"`
}
