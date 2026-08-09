package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/ramilamparo/hid2xbox/joystick"

	vigem "github.com/openstadia/go-vigem"
	x360 "github.com/openstadia/go-vigem/x360"
)

// ── Logger ─────────────────────────────────────────────────────────────────

func newZapLogger(path string) (*zap.Logger, func(), error) {
	if path == "" {
		return zap.NewNop(), func() {}, nil
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, nil, err
	}
	cfg := zap.NewProductionEncoderConfig()
	cfg.TimeKey = "ts"
	cfg.EncodeTime = zapcore.ISO8601TimeEncoder
	core := zapcore.NewCore(zapcore.NewJSONEncoder(cfg), zapcore.AddSync(f), zap.DebugLevel)
	l := zap.New(core)
	return l, func() { l.Sync(); f.Close() }, nil
}

// ── RunBridge ──────────────────────────────────────────────────────────────

// warnMappingConflicts prints warnings when multiple mappings target the same
// Xbox output (they will overwrite each other on the virtual controller).
func warnMappingConflicts(mappings []Mapping) {
	type key struct {
		target string
	}
	seen := map[key][]int{}
	for i, m := range mappings {
		k := key{m.Target}
		seen[k] = append(seen[k], i)
	}

	sticky := map[string]bool{
		"left_stick_x": true, "left_stick_y": true,
		"right_stick_x": true, "right_stick_y": true,
		"left_trigger": true, "right_trigger": true,
	}

	for k, idxs := range seen {
		if len(idxs) < 2 {
			continue
		}
		// Only warn for stick/trigger targets — buttons can be shared safely.
		if !sticky[k.target] {
			continue
		}
		var devs []string
		for _, i := range idxs {
			devs = append(devs, fmt.Sprintf("dev %d (axis %d)", mappings[i].Device, mappings[i].Source))
		}
		fmt.Fprintf(os.Stderr, "WARNING: %d mappings target %q — last one wins. Sources: %s\n",
			len(idxs), k.target, strings.Join(devs, ", "))
	}
}

func RunBridge(ctx context.Context, cfgPath string, connectMu *sync.Mutex, debugLogPath string, debugStdout bool) error {
	log, cleanup, err := newZapLogger(debugLogPath)
	if err != nil {
		return fmt.Errorf("debug logger: %w", err)
	}
	defer cleanup()

	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	devs := make([]*joystick.Device, len(cfg.Devices))
	for i, ds := range cfg.Devices {
		dev, err := findJoystick(ds)
		if err != nil {
			for j := 0; j < i; j++ {
				devs[j].Close()
			}
			return fmt.Errorf("device %d (%s): %w", i, ds.Name, err)
		}
		devs[i] = dev
		fmt.Printf("Device %d: %s (VID=%04X PID=%04X, %d axes, %d buttons)\n",
			i, dev.Info().Name, dev.Info().VID, dev.Info().PID,
			dev.Info().AxisCount, dev.Info().ButtonCount)
	}

	client := vigem.NewClient()
	defer client.Release()

	pad := x360.NewGamepad(client)
	connectMu.Lock()
	pad.Connect()
	connectMu.Unlock()
	defer func() {
		pad.Disconnect()
		pad.Release()
	}()

	fmt.Println("Virtual Xbox 360 controller connected. Press Ctrl+C to stop.")

	// Warn about mappings that overwrite each other.
	warnMappingConflicts(cfg.Mappings)

	if debugStdout {
		return runDebugLoop(ctx, devs, pad, cfg.Mappings)
	}

	return pollLoop(ctx, devs, pad, cfg.Mappings, log)
}

func findJoystick(ds DeviceSpec) (*joystick.Device, error) {
	if ds.ID > 0 {
		dev, err := joystick.Open(ds.ID)
		if err == nil {
			return dev, nil
		}
	}
	all := joystick.Enumerate()
	for _, info := range all {
		if contains(info.Name, ds.Name) {
			return joystick.Open(info.ID)
		}
	}
	return nil, fmt.Errorf("no joystick matching %q (id=%d) found among %d devices", ds.Name, ds.ID, len(all))
}

func contains(s, substr string) bool {
	return len(substr) == 0 || len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ── Poll loop ──────────────────────────────────────────────────────────────

func pollLoop(ctx context.Context, devs []*joystick.Device, pad *x360.Gamepad, mappings []Mapping, log *zap.Logger) error {
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := tick(devs, pad, mappings, log); err != nil {
				return err
			}
		}
	}
}

func tick(devs []*joystick.Device, pad *x360.Gamepad, mappings []Mapping, log *zap.Logger) error {
	type devState struct {
		axes    []uint32
		buttons uint32
		ok      bool
	}
	states := make([]devState, len(devs))

	devErrs := 0
	for i, dev := range devs {
		axes, buttons, err := dev.ReadRaw()
		if err != nil {
			devErrs++
			log.Error("device read error", zap.Int("dev", i), zap.Error(err))
			continue
		}
		states[i] = devState{axes: axes, buttons: buttons, ok: true}
	}
	if devErrs > 0 {
		log.Warn("devices skipped", zap.Int("count", devErrs))
	}

	pad.Reset()

	for _, m := range mappings {
		if m.Device < 0 || m.Device >= len(devs) {
			continue
		}
		devi := devs[m.Device]
		st := states[m.Device]
		if !st.ok {
			continue
		}

		switch m.Type {
		case "axis":
			if m.Source >= len(st.axes) {
				continue
			}
			raw := st.axes[m.Source]

			inMin := uint32(m.Min)
			inMax := uint32(m.Max)
			if inMin == 0 && inMax == 0 {
				dInfo := devi.Info()
				if m.Source < len(dInfo.AxisRanges) {
					inMin = dInfo.AxisRanges[m.Source].Min
					inMax = dInfo.AxisRanges[m.Source].Max
				}
			}

			mode := axisMode(m.Mode, m.Invert)

			// Axis → button via threshold.
			if _, isButton := buttonMap[m.Target]; isButton && m.Threshold > 0 {
				thr := m.Threshold
				if thr < 0.01 {
					thr = 0.01
				}
				if thr > 0.99 {
					thr = 0.99
				}
				v := clampNorm(raw, inMin, inMax, mode, m.Deadzone)
				if mode == "centered" || mode == "centered_inverted" {
					switch m.Direction {
					case "positive":
						if v < 0 {
							v = 0
						}
					case "negative":
						if v > 0 {
							v = 0
						} else {
							v = -v
						}
					default:
						if v < 0 {
							v = -v
						}
					}
				}
				pressed := v >= thr
				log.Debug("axis_to_button",
					zap.Int("dev", m.Device), zap.Int("axis", m.Source),
					zap.Float64("norm", v), zap.Float64("thr", thr),
					zap.Bool("pressed", pressed), zap.String("target", m.Target))
				applyButton(pad, m.Target, pressed)
				continue
			}

			applyAxis(pad, m.Target, raw, inMin, inMax, mode, m.Deadzone, m.Direction)
			norm := clampNorm(raw, inMin, inMax, mode, m.Deadzone)
			log.Debug("axis",
				zap.Int("dev", m.Device), zap.Int("axis", m.Source),
				zap.Uint32("raw", raw), zap.String("norm", fmt.Sprintf("%.6f", norm)),
				zap.String("mode", mode), zap.String("target", m.Target),
				zap.Int("deviation", int(raw)-32767),
				zap.Uint32("inMin", inMin), zap.Uint32("inMax", inMax))

		case "button":
			bit := uint32(1) << m.Source
			pressed := st.buttons&bit != 0
			applyButton(pad, m.Target, pressed)
			if pressed {
				log.Debug("button",
					zap.Int("dev", m.Device), zap.Int("btn", m.Source),
					zap.String("target", m.Target))
			}
		}
	}

	pad.Update()

	return nil
}

// ── Axis helpers ───────────────────────────────────────────────────────────

func axisMode(mode string, deprecatedInvert bool) string {
	if mode != "" {
		return mode
	}
	if deprecatedInvert {
		return "inverted"
	}
	return "normal"
}

func applyAxis(pad *x360.Gamepad, target string, raw, inMin, inMax uint32, mode string, deadzone float64, direction string) {
	if inMax == inMin {
		return
	}
	switch mode {
	case "inverted":
		raw = inMax - (raw - inMin)
	case "centered":
	case "centered_inverted":
		mid := (inMin + inMax) / 2
		if raw >= mid {
			raw = mid - (raw - mid)
		} else {
			raw = mid + (mid - raw)
		}
	}

	switch target {
	case "left_trigger", "right_trigger":
		v := axisToTrigger(raw, inMin, inMax, mode, deadzone, direction)
		if target == "left_trigger" {
			pad.LeftTrigger(v)
		} else {
			pad.RightTrigger(v)
		}
	default:
		x, y := axisToStick(target, raw, inMin, inMax, mode, deadzone, direction)
		switch target {
		case "left_stick_x":
			pad.LeftJoystick(x, 0)
		case "left_stick_y":
			pad.LeftJoystick(0, y)
		case "right_stick_x":
			pad.RightJoystick(x, 0)
		case "right_stick_y":
			pad.RightJoystick(0, y)
		}
	}
}

func axisToTrigger(raw, inMin, inMax uint32, mode string, deadzone float64, direction string) uint8 {
	v := clampNorm(raw, inMin, inMax, mode, deadzone)
	if mode == "centered" || mode == "centered_inverted" {
		if v < 0 {
			v = 0
		}
	}
	if direction == "negative" {
		v = 1 - v
	}
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	return uint8(v * 255)
}

func axisToStick(target string, raw, inMin, inMax uint32, mode string, deadzone float64, direction string) (int16, int16) {
	v := clampNorm(raw, inMin, inMax, mode, deadzone)
	if mode == "centered" || mode == "centered_inverted" {
		return int16(v * 32767), 0
	}
	switch direction {
	case "positive":
		return int16(v * 32767), 0
	case "negative":
		return int16(-(1-v) * 32767), 0
	default:
		return int16((v*2 - 1) * 32767), 0
	}
}

func clampNorm(raw, inMin, inMax uint32, mode string, deadzone float64) float64 {
	clamped := raw
	if clamped < inMin {
		clamped = inMin
	}
	if clamped > inMax {
		clamped = inMax
	}

	if mode == "centered" || mode == "centered_inverted" {
		mid := float64(inMin+inMax) / 2
		halfSpan := float64(inMax-inMin) / 2
		if halfSpan == 0 {
			return 0
		}
		v := (float64(clamped) - mid) / halfSpan
		if mode == "centered_inverted" {
			v = -v
		}
		if deadzone > 0 {
			if v > -deadzone && v < deadzone {
				v = 0
			} else if v < 0 {
				v = (v + deadzone) / (1 - deadzone)
			} else {
				v = (v - deadzone) / (1 - deadzone)
			}
		}
		return v
	}

	span := float64(inMax - inMin)
	v := float64(clamped-inMin) / span
	if mode == "inverted" {
		v = 1 - v
	}
	if deadzone > 0 {
		if v < deadzone {
			v = 0
		} else {
			v = (v - deadzone) / (1 - deadzone)
		}
	}
	return v
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

// ── Button map ─────────────────────────────────────────────────────────────

var buttonMap = map[string]x360.XUSB_BUTTON{
	"a": x360.XUSB_GAMEPAD_A, "b": x360.XUSB_GAMEPAD_B,
	"x": x360.XUSB_GAMEPAD_X, "y": x360.XUSB_GAMEPAD_Y,
	"lb": x360.XUSB_GAMEPAD_LEFT_SHOULDER, "rb": x360.XUSB_GAMEPAD_RIGHT_SHOULDER,
	"lthumb": x360.XUSB_GAMEPAD_LEFT_THUMB, "rthumb": x360.XUSB_GAMEPAD_RIGHT_THUMB,
	"start": x360.XUSB_GAMEPAD_START, "back": x360.XUSB_GAMEPAD_BACK,
	"dup": x360.XUSB_GAMEPAD_DPAD_UP, "ddown": x360.XUSB_GAMEPAD_DPAD_DOWN,
	"dleft": x360.XUSB_GAMEPAD_DPAD_LEFT, "dright": x360.XUSB_GAMEPAD_DPAD_RIGHT,
	"left_shoulder": x360.XUSB_GAMEPAD_LEFT_SHOULDER, "right_shoulder": x360.XUSB_GAMEPAD_RIGHT_SHOULDER,
	"left_thumb": x360.XUSB_GAMEPAD_LEFT_THUMB, "right_thumb": x360.XUSB_GAMEPAD_RIGHT_THUMB,
	"dpad_up": x360.XUSB_GAMEPAD_DPAD_UP, "dpad_down": x360.XUSB_GAMEPAD_DPAD_DOWN,
	"dpad_left": x360.XUSB_GAMEPAD_DPAD_LEFT, "dpad_right": x360.XUSB_GAMEPAD_DPAD_RIGHT,
}

func signalContext() context.Context {
	ctx, _ := signal.NotifyContext(context.Background(), os.Interrupt)
	return ctx
}
