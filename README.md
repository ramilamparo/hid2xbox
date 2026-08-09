# hid2xbox

Map any generic HID gamepad or handbrake to a virtual Xbox 360 controller on Windows.

Use case: your Arduino Leonardo handbrake is detected by Windows but games don't see it because they expect XInput. `hid2xbox` reads the raw joystick and feeds it into a virtual Xbox 360 controller via [ViGEmBus](https://github.com/nefarius/ViGEmBus).

## Prerequisites

1. **[ViGEmBus](https://github.com/nefarius/ViGEmBus/releases)** — virtual gamepad driver. Install once.
2. **[ViGEmClient.dll](https://github.com/nefarius/ViGEmClient)** — drop next to `hid2xbox.exe`, or in `PATH`.
3. **[HidHide](https://github.com/nefarius/HidHide)** *(recommended)* — hides the physical device from games so they only see the virtual Xbox controller. Without it, games receive double input.

## Install

### Prebuilt (recommended)

Download `hid2xbox.exe` from [releases](https://github.com/ramilamparo/hid2xbox/releases). Place `ViGEmClient.dll` in the same folder.

### Build from source

```powershell
# Clone and build
git clone https://github.com/ramilamparo/hid2xbox.git
cd hid2xbox
go build -o hid2xbox.exe .

# Or install to %GOPATH%\bin
go install github.com/ramilamparo/hid2xbox@latest
```

Requires Go 1.22+.

## Quick Start

### 1. Discover your device

```powershell
.\hid2xbox.exe --setup
```

This launches an interactive TUI:

1. **Select** your controller from the list (e.g. "Arduino Leonardo")
2. **Discover** — pull your handbrake or press buttons to see which inputs move
3. **Map** each input to an Xbox target (e.g. Axis 2 → right_trigger)
4. **Save** to `config.json`

### 2. Run the bridge

```powershell
.\hid2xbox.exe
```

Press `Ctrl+C` to stop. The virtual Xbox controller disconnects cleanly.

### 3. Prevent double input (one-time setup)

Games see both the physical Arduino *and* the virtual Xbox controller. Install [HidHide](https://github.com/nefarius/HidHide), open HidHide Configuration Client, and:

- **Devices tab**: check your Arduino to hide it from everything
- **Applications tab**: add `hid2xbox.exe` to the whitelist
- Enable "Enable device hiding"

Now only `hid2xbox` can read the Arduino. Games see only the virtual Xbox controller.

## Configuration

`config.json` is created by `--setup` or written by hand:

```json
{
  "name": "Arduino Leonardo",
  "mappings": [
    {
      "type": "axis",
      "source": 2,
      "target": "right_trigger",
      "min": 0,
      "max": 1023,
      "invert": false,
      "deadzone": 0.05
    },
    {
      "type": "button",
      "source": 0,
      "target": "a"
    }
  ]
}
```

| Field | Description |
|---|---|
| `name` | Substring match against the Windows joystick name |
| `type` | `"axis"` or `"button"` |
| `source` | Axis index (0–5) or button bit index |
| `target` | Xbox output (see below) |

**Axis targets:** `left_trigger`, `right_trigger`, `left_stick_x`, `left_stick_y`, `right_stick_x`, `right_stick_y`

**Button targets:** `a`, `b`, `x`, `y`, `left_shoulder`, `right_shoulder`, `start`, `back`, `left_thumb`, `right_thumb`, `dpad_up`, `dpad_down`, `dpad_left`, `dpad_right`

## CLI Flags

| Flag | Description |
|---|---|
| `--config <path>` | Path to config file (default: `config.json`) |
| `--setup` | Launch the interactive TUI to create a config |

## Testing

```powershell
# All tests (needs Windows + attached joysticks for TUI tests)
go test -v -count=1 ./...

# Portable tests only (cross-platform, no hardware needed)
go test -v -count=1 -run "Test(Config|Normalize|Norm|Button|Contain)" ./...
```

## How It Works

```
Arduino Leonardo ──→ joyGetPosEx (Winmm.dll) ──→ hid2xbox ──→ ViGEmBus ──→ Virtual Xbox 360
                         │                                                       │
                    HidHide blocks                                              Game sees
                    games from seeing                                         only this
                    this directly
```

## License

MIT — see [LICENSE](LICENSE).
