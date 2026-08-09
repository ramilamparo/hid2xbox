package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// DeviceSpec identifies a physical joystick by name and/or stable ID.
type DeviceSpec struct {
	// Name is a substring match against the joystick name reported by Windows.
	Name string `json:"name"`
}

// Config is the top-level configuration for one hid2xbox bridge instance.
type Config struct {
	// Devices is a list of physical joysticks feeding into the virtual controller.
	Devices []DeviceSpec `json:"devices,omitempty"`

	// Mappings define how joystick inputs translate to Xbox controller outputs.
	Mappings []Mapping `json:"mappings"`
}

// Mapping describes a single joystick-to-xbox binding.
type Mapping struct {
	// Device is the index into Config.Devices (defaults to 0).
	Device int `json:"device,omitempty"`

	// Type is "axis" or "button".
	Type string `json:"type"`

	// Source is the joystick axis index (0-based) or button bit index.
	Source int `json:"source"`

	// Target is the Xbox output.
	Target string `json:"target"`

	// Min and Max define the input range for axis mappings.
	Min int `json:"min,omitempty"`
	Max int `json:"max,omitempty"`

	// Mode determines how the raw axis is interpreted:
	//   "normal"            — 0..max → 0..1 (handbrake, throttle)
	//   "inverted"          — 0..max → 1..0
	//   "centered"          — min..mid..max → -1..0..+1 (steering wheel)
	//   "centered_inverted" — min..mid..max → +1..0..-1
	Mode string `json:"mode,omitempty"`

	// Deadzone is a fraction (0.0–1.0) of the full range ignored near center/zero.
	Deadzone float64 `json:"deadzone,omitempty"`

	// Threshold activates a button press when the normalized axis value crosses
	// this point (0.0–1.0). Only meaningful when type is "axis" and target is a
	// button name. 0 means no threshold triggering.
	Threshold float64 `json:"threshold,omitempty"`

	// Direction limits a stick mapping to one half: "both" (default), "positive", "negative".
	Direction string `json:"direction,omitempty"`
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
