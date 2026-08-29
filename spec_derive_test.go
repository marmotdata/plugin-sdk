package pluginsdk

import (
	"testing"
)

type deriveTestConfig struct {
	BootstrapServers string            `json:"bootstrap_servers" description:"Bootstrap servers" validate:"required"`
	ClientID         string            `json:"client_id" description:"Client ID"`
	Authentication   *deriveAuthConfig `json:"authentication,omitempty" description:"Auth"`
	TLS              *deriveTLSConfig  `json:"tls,omitempty" description:"TLS"`
	ConsumerConfig   map[string]string `json:"consumer_config,omitempty" description:"Consumer config"`
}

type deriveAuthConfig struct {
	Type      string `json:"type" description:"Auth type" validate:"omitempty,oneof=none sasl_ssl"`
	Username  string `json:"username,omitempty" description:"Username"`
	Password  string `json:"password,omitempty" description:"Password" sensitive:"true"`
	Mechanism string `json:"mechanism,omitempty" description:"Mechanism"`
}

type deriveTLSConfig struct {
	Enabled bool `json:"enabled" description:"TLS enabled"`
}

func findField(spec []ConfigField, name string) *ConfigField {
	for i := range spec {
		if spec[i].Name == name {
			return &spec[i]
		}
	}
	return nil
}

func TestDeriveSpec_NoOptionsReturnsFullSpec(t *testing.T) {
	spec := DeriveSpec(deriveTestConfig{})

	if findField(spec, "bootstrap_servers") == nil {
		t.Fatal("expected bootstrap_servers in spec")
	}
	if findField(spec, "tls") == nil {
		t.Fatal("expected tls in spec")
	}
}

func TestDeriveSpec_HideRemovesTopLevel(t *testing.T) {
	spec := DeriveSpec(deriveTestConfig{}, Hide("tls", "consumer_config"))

	if findField(spec, "tls") != nil {
		t.Error("tls should have been removed")
	}
	if findField(spec, "consumer_config") != nil {
		t.Error("consumer_config should have been removed")
	}
	if findField(spec, "bootstrap_servers") == nil {
		t.Error("bootstrap_servers should still be present")
	}
}

func TestDeriveSpec_HideRemovesNested(t *testing.T) {
	spec := DeriveSpec(deriveTestConfig{}, Hide("authentication.type", "authentication.mechanism"))

	auth := findField(spec, "authentication")
	if auth == nil {
		t.Fatal("authentication should still be present")
	}
	if findField(auth.Fields, "type") != nil {
		t.Error("authentication.type should have been removed")
	}
	if findField(auth.Fields, "mechanism") != nil {
		t.Error("authentication.mechanism should have been removed")
	}
	if findField(auth.Fields, "username") == nil {
		t.Error("authentication.username should still be present")
	}
}

func TestDeriveSpec_HidePanicsOnUnknownField(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on unknown field")
		}
	}()
	DeriveSpec(deriveTestConfig{}, Hide("does_not_exist"))
}

func TestDeriveSpec_OverridePlaceholder(t *testing.T) {
	spec := DeriveSpec(deriveTestConfig{},
		Override("bootstrap_servers", Placeholder("example:9092")),
	)

	field := findField(spec, "bootstrap_servers")
	if field == nil {
		t.Fatal("bootstrap_servers not found")
	}
	if field.Placeholder != "example:9092" {
		t.Errorf("expected placeholder %q, got %q", "example:9092", field.Placeholder)
	}
}

func TestDeriveSpec_OverridePanicsOnUnknownField(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on unknown field")
		}
	}()
	DeriveSpec(deriveTestConfig{}, Override("does_not_exist", Placeholder("x")))
}

func TestDeriveSpec_OptionsApplyInOrder(t *testing.T) {
	// Hide first, then Override on a surviving field.
	spec := DeriveSpec(deriveTestConfig{},
		Hide("tls"),
		Override("client_id", Placeholder("marmot")),
	)

	if findField(spec, "tls") != nil {
		t.Error("tls should have been removed")
	}
	if field := findField(spec, "client_id"); field == nil || field.Placeholder != "marmot" {
		t.Errorf("expected client_id placeholder %q, got %+v", "marmot", field)
	}
}
