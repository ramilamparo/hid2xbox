package main
import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/ramilamparo/hid2xbox/joystick"

	vigem "github.com/openstadia/go-vigem"
	x360 "github.com/openstadia/go-vigem/x360"
)

// RunBridge loads a config, finds the matching joystick, creates a virtual
// Xbox 360 controller, and runs the poll loop until the context is cancelled.
func RunBridge(ctx context.Context, cfgPath string) error {
	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	dev, err := findJoystick(cfg.Name)
	if err != nil {
		return fmt.Errorf("finding joystick %q: %w", cfg.Name, err)
	}
	fmt.Printf("Found joystick: %s (VID=%04X PID=%04X, %d axes, %d buttons)\n",
		dev.Info().Name, dev.Info().VID, dev.Info().PID,
		dev.Info().AxisCount, dev.Info().ButtonCount)

	client := vigem.NewClient()
	defer client.Release()

	pad := x360.NewGamepad(client)
	pad.Connect()
	defer func() {
		pad.Disconnect()
		pad.Release()
	}()

	fmt.Println("Virtual Xbox 360 controller connected. Press Ctrl+C to stop.")


	return pollLoop(ctx, dev, pad, cfg.Mappings)
}

func findJoystick(name string) (*joystick.Device, error) {
	all := joystick.Enumerate()
	for _, info := range all {
		if contains(info.Name, name) {
			return joystick.Open(info.ID)
		}
	}
	return nil, fmt.Errorf("no joystick matching %q found (scanned %d devices)", name, len(all))
}

func contains(s, substr string) bool {
	return len(substr) == 0 || len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := range len(s) - len(substr) + 1 {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func pollLoop(ctx context.Context, dev *joystick.Device, pad *x360.Gamepad, mappings []Mapping) error {
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := tick(dev, pad, mappings); err != nil {
				return err
			}
		}
	}
}

func tick(dev *joystick.Device, pad *x360.Gamepad, mappings []Mapping) error {
	axes, buttons, err := dev.ReadRaw()
	if err != nil {
		return fmt.Errorf("reading joystick: %w", err)
	}

	// Reset to zero before applying each mapping — the callers that
	// set values will override. This keeps the controller clean when
	// no mapping targets a particular control.
	pad.Reset()

	for _, m := range mappings {
		switch m.Type {
		case "axis":
			if m.Source >= len(axes) {
				continue
			}
			raw := axes[m.Source]

			// Determine input range.
			devInfo := dev.Info()
			inMin := uint32(m.Min)
			inMax := uint32(m.Max)
			if inMin == 0 && inMax == 0 && m.Source < len(devInfo.AxisRanges) {
				inMin = devInfo.AxisRanges[m.Source].Min
				inMax = devInfo.AxisRanges[m.Source].Max
			}

			applyAxis(pad, m.Target, raw, inMin, inMax, m.Invert, m.Deadzone)

		case "button":
			bit := uint32(1) << m.Source
			pressed := buttons&bit != 0
			applyButton(pad, m.Target, pressed)
		}
	}

	pad.Update()
	return nil
}

func applyAxis(pad *x360.Gamepad, target string, raw, inMin, inMax uint32, invert bool, deadzone float64) {
	norm := normalizeAxis(raw, inMin, inMax, invert, deadzone)
	if norm < 0 {
		return
	}
	switch target {
	case "left_trigger":
		pad.LeftTrigger(uint8(norm * 255))
	case "right_trigger":
		pad.RightTrigger(uint8(norm * 255))
	case "left_stick_x":
		pad.LeftJoystick(normToStick(norm), 0)
	case "left_stick_y":
		pad.LeftJoystick(0, normToStick(norm))
	case "right_stick_x":
		pad.RightJoystick(normToStick(norm), 0)
	case "right_stick_y":
		pad.RightJoystick(0, normToStick(norm))
	}
}

// normalizeAxis converts a raw axis value to a 0.0–1.0 float.
// Returns -1 if inMax == inMin (invalid range).
func normalizeAxis(raw, inMin, inMax uint32, invert bool, deadzone float64) float64 {
	if inMax == inMin {
		return -1
	}
	clamped := raw
	if clamped < inMin {
		clamped = inMin
	}
	if clamped > inMax {
		clamped = inMax
	}
	span := float64(inMax - inMin)
	norm := float64(clamped-inMin) / span

	if deadzone > 0 {
		if norm < deadzone {
			norm = 0
		} else {
			norm = (norm - deadzone) / (1 - deadzone)
		}
	}
	if invert {
		norm = 1 - norm
	}
	return norm
}

func normToStick(norm float64) int16 {
	return int16((norm*2 - 1) * 32767)
}

func applyButton(pad *x360.Gamepad, target string, pressed bool) {
	bit, ok := buttonMap[target]
	if !ok {
		return
	}
	if pressed {
		pad.PressButton(bit)
	} else {
		pad.ReleaseButton(bit)
	}
}

var buttonMap = map[string]x360.XUSB_BUTTON{
	"a":              x360.XUSB_GAMEPAD_A,
	"b":              x360.XUSB_GAMEPAD_B,
	"x":              x360.XUSB_GAMEPAD_X,
	"y":              x360.XUSB_GAMEPAD_Y,
	"left_shoulder":  x360.XUSB_GAMEPAD_LEFT_SHOULDER,
	"right_shoulder": x360.XUSB_GAMEPAD_RIGHT_SHOULDER,
	"start":          x360.XUSB_GAMEPAD_START,
	"back":           x360.XUSB_GAMEPAD_BACK,
	"left_thumb":     x360.XUSB_GAMEPAD_LEFT_THUMB,
	"right_thumb":    x360.XUSB_GAMEPAD_RIGHT_THUMB,
	"dpad_up":        x360.XUSB_GAMEPAD_DPAD_UP,
	"dpad_down":      x360.XUSB_GAMEPAD_DPAD_DOWN,
	"dpad_left":      x360.XUSB_GAMEPAD_DPAD_LEFT,
	"dpad_right":     x360.XUSB_GAMEPAD_DPAD_RIGHT,
}

func signalContext() context.Context {
	ctx, _ := signal.NotifyContext(context.Background(), os.Interrupt)
	return ctx
}
