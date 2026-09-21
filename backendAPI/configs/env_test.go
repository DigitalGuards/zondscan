package configs

import (
	"errors"
	"testing"
)

func TestLoadExplainAuthSettingsRequiresPairedValues(t *testing.T) {
	tests := []struct {
		name   string
		values map[string]string
	}{
		{name: "both absent", values: map[string]string{}},
		{name: "origin only", values: map[string]string{explainAuthOriginEnv: "https://zondscan.com"}},
		{name: "chain only", values: map[string]string{explainAuthChainIDEnv: "0x539"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := LoadExplainAuthSettings(func(key string) string { return test.values[key] })
			if !errors.Is(err, ErrExplainAuthNotConfigured) {
				t.Fatalf("error = %v, want ErrExplainAuthNotConfigured", err)
			}
		})
	}
}

func TestLoadExplainAuthSettingsTrimsConfiguredValues(t *testing.T) {
	values := map[string]string{
		explainAuthOriginEnv:  "  https://zondscan.com  ",
		explainAuthChainIDEnv: "  0x539  ",
	}
	settings, err := LoadExplainAuthSettings(func(key string) string { return values[key] })
	if err != nil {
		t.Fatalf("LoadExplainAuthSettings: %v", err)
	}
	if settings.Origin != "https://zondscan.com" || settings.ExpectedChainID != "0x539" {
		t.Fatalf("settings = %#v", settings)
	}
}
