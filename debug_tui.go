package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ramilamparo/hid2xbox/joystick"
	x360 "github.com/openstadia/go-vigem/x360"
)

func runDebugLoop(ctx context.Context, devs []*joystick.Device, pad *x360.Gamepad, mappings []Mapping) error {
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()

	fmt.Print("\033[2J\033[?25l")
	defer fmt.Print("\033[?25h")

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}

		// Read devices.
		type devState struct {
			axes    []uint32
			buttons uint32
			ok      bool
		}
		states := make([]devState, len(devs))
		for i, dev := range devs {
			axes, buttons, err := dev.ReadRaw()
			if err != nil {
				continue
			}
			states[i] = devState{axes: axes, buttons: buttons, ok: true}
		}

		pad.Reset()

		// Build per-mapping display lines.
		type line struct{ text string }
		var lines []line

		for _, mp := range mappings {
			if mp.Device < 0 || mp.Device >= len(devs) {
				continue
			}
			devi := devs[mp.Device]
			st := states[mp.Device]
			if !st.ok {
				continue
			}
			switch mp.Type {
			case "axis":
				if mp.Source >= len(st.axes) {
					continue
				}
				raw := st.axes[mp.Source]
				inMin := uint32(mp.Min)
				inMax := uint32(mp.Max)
				if inMin == 0 && inMax == 0 {
					dInfo := devi.Info()
					if mp.Source < len(dInfo.AxisRanges) {
						inMin = dInfo.AxisRanges[mp.Source].Min
						inMax = dInfo.AxisRanges[mp.Source].Max
					}
				}
				mode := axisMode(mp.Mode, mp.Invert)

				// Threshold path.
				if _, isBtn := buttonMap[mp.Target]; isBtn && mp.Threshold > 0 {
					thr := mp.Threshold
					if thr < 0.01 {
						thr = 0.01
					}
					if thr > 0.99 {
						thr = 0.99
					}
					v := clampNorm(raw, inMin, inMax, mode, mp.Deadzone)
					if mode == "centered" || mode == "centered_inverted" {
						switch mp.Direction {
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
					applyButton(pad, mp.Target, v >= thr)
					continue
				}

				applyAxis(pad, mp.Target, raw, inMin, inMax, mode, mp.Deadzone, mp.Direction)

				// Compute display value.
				norm := clampNorm(raw, inMin, inMax, mode, mp.Deadzone)
				val := ""
				switch {
				case mp.Target == "left_trigger" || mp.Target == "right_trigger":
					tv := axisToTrigger(raw, inMin, inMax, mode, mp.Deadzone, mp.Direction)
					val = fmt.Sprintf("%3d", tv)
				case strings.HasSuffix(mp.Target, "_x") || strings.HasSuffix(mp.Target, "_y"):
					x, y := axisToStick(mp.Target, raw, inMin, inMax, mode, mp.Deadzone, mp.Direction)
					if strings.HasSuffix(mp.Target, "_x") {
						val = fmt.Sprintf("%+6d", x)
					} else {
						val = fmt.Sprintf("%+6d", y)
					}
				}
				lines = append(lines, line{
					text: fmt.Sprintf("  d%d.a%-2d → %-16s  raw=%-6d  norm=%+.6f  out=%s",
						mp.Device, mp.Source, mp.Target, raw, norm, val),
				})

			case "button":
				bit := uint32(1) << mp.Source
				pressed := st.buttons&bit != 0
				applyButton(pad, mp.Target, pressed)
				if pressed {
					lines = append(lines, line{text: fmt.Sprintf("  d%d.b%-2d → %-16s  PRESSED", mp.Device, mp.Source, mp.Target)})
				}
			}
		}

		pad.Update()

		// Render.
		fmt.Print("\033[H")
		fmt.Println("  hid2xbox — Debug")
		for _, l := range lines {
			fmt.Println(l.text)
		}
		fmt.Println()

		// Raw axes.
		for di, st := range states {
			if !st.ok || len(st.axes) == 0 {
				continue
			}
			var parts []string
			for ai, v := range st.axes {
				parts = append(parts, fmt.Sprintf("a%d:%5d", ai, v))
			}
			fmt.Printf("  Dev %d: %s\n", di, strings.Join(parts, "  "))
		}
		fmt.Println("  Ctrl+C to quit")
	}
}
