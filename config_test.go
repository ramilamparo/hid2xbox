package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	cfg, err := LoadConfig("testdata/valid.json")
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Name != "Arduino Leonardo" {
		t.Errorf("Name = %q, want %q", cfg.Name, "Arduino Leonardo")
	}
	if len(cfg.Mappings) != 2 {
		t.Fatalf("got %d mappings, want 2", len(cfg.Mappings))
	}

	// Axis mapping
	m0 := cfg.Mappings[0]
	if m0.Type != "axis" || m0.Source != 2 || m0.Target != "right_trigger" {
		t.Errorf("mapping[0] = %+v, want axis 2→right_trigger", m0)
	}
	if m0.Min != 0 || m0.Max != 1023 {
		t.Errorf("mapping[0] range = %d–%d, want 0–1023", m0.Min, m0.Max)
	}
	if !m0.Invert {
		t.Error("mapping[0] invert should be true")
	}

	// Button mapping
	m1 := cfg.Mappings[1]
	if m1.Type != "button" || m1.Source != 0 || m1.Target != "a" {
		t.Errorf("mapping[1] = %+v, want button 0→a", m1)
	}
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	_, err := LoadConfig("testdata/nonexistent.json")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadConfig_InvalidJSON(t *testing.T) {
	_, err := LoadConfig("testdata/invalid.json")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestSaveAndLoadRoundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg := &Config{
		Name: "Test Device",
		Mappings: []Mapping{
			{Type: "axis", Source: 1, Target: "left_trigger", Min: 0, Max: 255, Invert: true, Deadzone: 0.05},
			{Type: "button", Source: 3, Target: "b"},
		},
	}

	if err := SaveConfig(path, cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	loaded, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if loaded.Name != cfg.Name {
		t.Errorf("Name = %q, want %q", loaded.Name, cfg.Name)
	}
	if len(loaded.Mappings) != len(cfg.Mappings) {
		t.Fatalf("got %d mappings, want %d", len(loaded.Mappings), len(cfg.Mappings))
	}
	for i := range cfg.Mappings {
		if loaded.Mappings[i] != cfg.Mappings[i] {
			t.Errorf("mapping[%d]: got %+v, want %+v", i, loaded.Mappings[i], cfg.Mappings[i])
		}
	}

	// Verify JSON is pretty-printed (has newline at end).
	data, _ := os.ReadFile(path)
	if len(data) == 0 || data[len(data)-1] != '\n' {
		t.Error("saved config should end with newline")
	}
	var raw json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Errorf("saved config is not valid JSON: %v", err)
	}
}

func TestConfig_EmptyMappings(t *testing.T) {
	cfg := &Config{Name: "Empty", Mappings: nil}
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.json")
	if err := SaveConfig(path, cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	loaded, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if loaded.Name != "Empty" || loaded.Mappings != nil {
		t.Errorf("got Name=%q Mappings=%v, want Name=Empty Mappings=nil", loaded.Name, loaded.Mappings)
	}
}
