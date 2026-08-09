package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config is the top-level configuration for one hid2xbox bridge instance.
type Config struct {
	// Name is a substring match against the joystick name reported by Windows.
	Name string `json:"name"`

	// ID is the stable joystick identifier from joyGetDevCaps (0-15).
	// When set, it takes priority over name matching.
	ID int `json:"id,omitempty"`

	// Mappings define how joystick inputs translate to Xbox controller outputs.
	Mappings []Mapping `json:"mappings"`
}

// Mapping describes a single joystick-to-xbox binding.
type Mapping struct {
	// Type is "axis" or "button".
	Type string `json:"type"`

	// Source is the joystick axis index (0-based) or button bit index.
	Source int `json:"source"`

	// Target is the Xbox output: "a", "b", "x", "y", "left_shoulder",
	// "right_shoulder", "start", "back", "left_thumb", "right_thumb",
	// "dpad_up", "dpad_down", "dpad_left", "dpad_right",
	// "left_trigger", "right_trigger",
	// "left_stick_x", "left_stick_y", "right_stick_x", "right_stick_y".
	Target string `json:"target"`

	// Min and Max define the input range for axis mappings.
	// For buttons these are ignored.
	Min int `json:"min,omitempty"`
	Max int `json:"max,omitempty"`

	// Invert flips the axis direction (255−value for triggers, −value for sticks).
	Invert bool `json:"invert,omitempty"`

	// Deadzone is a fraction (0.0–1.0) of the full range ignored near center/zero.
	Deadzone float64 `json:"deadzone,omitempty"`
}

// LoadConfig reads a JSON config file.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	return &cfg, nil
}

// SaveConfig writes a JSON config file.
func SaveConfig(path string, cfg *Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling config: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}
	return nil
}
