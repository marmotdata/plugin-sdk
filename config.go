package pluginsdk

import (
	"fmt"

	"sigs.k8s.io/yaml"
)

// UnmarshalConfig unmarshals a raw config into a specific plugin config type.
func UnmarshalConfig[T any](raw RawConfig) (*T, error) {
	data, err := yaml.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("re-marshaling config: %w", err)
	}

	var config T
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("unmarshaling into plugin config: %w", err)
	}

	return &config, nil
}
