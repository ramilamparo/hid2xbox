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
	if len(cfg.Devices) != 1 {
		t.Fatalf("got %d devices, want 1", len(cfg.Devices))
	}
	if cfg.Devices[0].Name != "Arduino Leonardo" {
		t.Errorf("Devices[0].Name = %q, want %q", cfg.Devices[0].Name, "Arduino Leonardo")
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
	if m0.Mode != "inverted" {
		t.Errorf("mapping[0] mode = %q, want inverted", m0.Mode)
	}

	// Button mapping
	m1 := cfg.Mappings[1]
	if m1.Type != "button" || m1.Source != 0 || m1.Target != "a" {
		t.Errorf("mapping[1] = %+v, want button 0→a", m1)
	}
}

func TestConfig_MultiDevice(t *testing.T) {
	cfg, err := LoadConfig("testdata/multi.json")
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if len(cfg.Devices) != 2 {
		t.Fatalf("got %d devices, want 2", len(cfg.Devices))
	}
	if cfg.Devices[0].Name != "PXN-V12lite" {
		t.Errorf("device[0] = %q", cfg.Devices[0].Name)
	}
	if cfg.Devices[1].Name != "Arduino Leonardo" {
		t.Errorf("device[1] = %q", cfg.Devices[1].Name)
	}
	if len(cfg.Mappings) != 2 {
		t.Fatalf("got %d mappings, want 2", len(cfg.Mappings))
	}
	if cfg.Mappings[0].Device != 0 || cfg.Mappings[1].Device != 1 {
		t.Errorf("mapping devices: %d, %d", cfg.Mappings[0].Device, cfg.Mappings[1].Device)
	}
}

func TestConfig_SaveLoadRoundtrip(t *testing.T) {
	cfg := &Config{
		Devices: []DeviceSpec{
			{Name: "Test Device"},
		},
		Mappings: []Mapping{
			{Type: "axis", Source: 1, Target: "left_trigger", Min: 0, Max: 255, Mode: "inverted", Deadzone: 0.05},
			{Type: "button", Source: 3, Target: "b"},
		},
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	if err := SaveConfig(path, cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	loaded, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if len(loaded.Devices) != 1 || loaded.Devices[0].Name != "Test Device" {
		t.Errorf("device not roundtripped")
	}
	if len(loaded.Mappings) != 2 {
		t.Fatalf("got %d mappings, want 2", len(loaded.Mappings))
	}

	m0 := loaded.Mappings[0]
	if m0.Mode != "inverted" {
		t.Errorf("mode = %q, want inverted", m0.Mode)
	}
	if m0.Deadzone != 0.05 {
		t.Errorf("deadzone = %v", m0.Deadzone)
	}
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	_, err := LoadConfig("testdata/nonexistent.json")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoadConfig_InvalidJSON(t *testing.T) {
	_, err := LoadConfig("testdata/invalid.json")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestConfig_EmptyMappings(t *testing.T) {
	cfg := &Config{
		Devices:  []DeviceSpec{{Name: "Device"}},
		Mappings: []Mapping{},
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var back Config
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}
	if len(back.Mappings) != 0 {
		t.Errorf("got %d mappings, want 0", len(back.Mappings))
	}
}

func TestSaveConfig_WriteAndRead(t *testing.T) {
	cfg := &Config{
		Devices:  []DeviceSpec{{Name: "Test"}},
		Mappings: []Mapping{{Type: "axis", Source: 0, Target: "left_trigger"}},
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "out.json")
	if err := SaveConfig(path, cfg); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(data) {
		t.Error("output is not valid JSON")
	}
}
