# hid2xbox

Map any generic HID gamepad, handbrake, or direct-drive wheel to a virtual Xbox 360 controller on Windows.

Use case: your Arduino Leonardo handbrake + PXN direct-drive wheel are detected by Windows but games don't see them because they expect XInput. `hid2xbox` reads raw joystick inputs and feeds them into a virtual Xbox 360 controller via [ViGEmBus](https://github.com/nefarius/ViGEmBus).

## Prerequisites

1. **[ViGEmBus](https://github.com/nefarius/ViGEmBus/releases)** — virtual gamepad driver. Install once.
2. **[ViGEmClient.dll](https://github.com/nefarius/ViGEmClient)** — drop next to `hid2xbox.exe`, or in `PATH`.

## Install

### Prebuilt (recommended)

Download `hid2xbox.exe` and `ViGEmClient.dll` from [releases](https://github.com/ramilamparo/hid2xbox/releases). Place both in the same folder.

### Build from source

```powershell
git clone https://github.com/ramilamparo/hid2xbox.git
cd hid2xbox
go build -o hid2xbox.exe .

# Install to %GOPATH%\bin (requires Go 1.23+)
go install github.com/ramilamparo/hid2xbox@latest
```

## Quick Start

### 1. Configure your devices

```powershell
.\hid2xbox.exe --setup
```

This launches an interactive TUI:

1. **Select** a controller from the list (e.g. "PXN-V12lite")
2. **Discover** — move axes or press buttons to see which inputs respond
3. **Map** each input to an Xbox target
4. Press **`tab`** to switch devices and map additional controllers
5. **Save** — all devices written to `config.json`

### 2. Run the bridge

```powershell
.\hid2xbox.exe
```

Press `Ctrl+C` to stop. The virtual Xbox controller disconnects cleanly.

**Multiple controllers feeding the same virtual gamepad:**

```powershell
.\hid2xbox.exe --config config_pxn.json --config config_arduino.json
```

**Live debug view:**

```powershell
.\hid2xbox.exe --debug
# or with file logging:
.\hid2xbox.exe --debug-log .\debug.jsonl
```

## Configuration

`config.json` is created by `--setup` or written by hand:

```json
{
  "devices": [
    { "name": "PXN-V12lite", "id": 4 },
    { "name": "Arduino Leonardo", "id": 3 }
  ],
  "mappings": [
    {
      "device": 0,
      "type": "axis",
      "source": 0,
      "target": "right_stick_x",
      "max": 65535,
      "mode": "centered",
      "deadzone": 0.05,
      "direction": "both"
    },
    {
      "device": 0,
      "type": "axis",
      "source": 3,
      "target": "left_trigger",
      "max": 65535,
      "mode": "normal",
      "deadzone": 0.05
    },
    {
      "device": 1,
      "type": "axis",
      "source": 3,
      "target": "right_trigger",
      "max": 65535,
      "mode": "normal",
      "deadzone": 0.05
    }
  ]
}
```

| Field | Description |
|---|---|
| `devices` | Array of physical joysticks. Each has a `name` (substring match) and optional `id` |
| `device` | Index into `devices` — which physical controller this mapping belongs to |
| `type` | `"axis"` or `"button"` |
| `source` | Axis index (0-based) or button bit index |
| `target` | Xbox output (see below) |
| `min` / `max` | Input range override for axis mappings (default: hardware range) |
| `mode` | Axis interpretation:<br>`"normal"` — 0→max maps to 0→1 (handbrake, throttle)<br>`"inverted"` — 0→max maps to 1→0<br>`"centered"` — min←mid→max maps to -1←0→+1 (steering wheel)<br>`"centered_inverted"` — min←mid→max maps to +1←0→-1 |
| `direction` | Stick half-range: `"both"` (default), `"positive"` (0→+1), `"negative"` (0→-1) |
| `threshold` | Axis→button: value 0.0–1.0 where axis triggers a button press |
| `deadzone` | Fraction (0.0–1.0) ignored near center/zero |
| `invert` | **Deprecated** — use `mode: "inverted"` instead |

**Axis targets:** `left_trigger`, `right_trigger`, `left_stick_x`, `left_stick_y`, `right_stick_x`, `right_stick_y`

**Button targets:** `a`, `b`, `x`, `y`, `left_shoulder`, `right_shoulder`, `start`, `back`, `left_thumb`, `right_thumb`, `dpad_up`, `dpad_down`, `dpad_left`, `dpad_right`

## CLI Flags

| Flag | Description |
|---|---|
| `--config <path>` | Path to config file (repeatable for multi-controller) |
| `--setup` | Interactive TUI to discover inputs and create a config |
| `--debug` | Live virtual Xbox controller state display |
| `--debug-log <path>` | Write structured JSONL debug logs to file |

## Testing

```powershell
go test -v -count=1 ./...
```

## How It Works

```
PXN-V12lite ──→ joyGetPosEx (Winmm.dll) ──╮
Arduino Leonardo ──→ joyGetPosEx ──────────╂──→ hid2xbox ──→ ViGEmBus ──→ Virtual Xbox 360
```

## License

MIT — see [LICENSE](LICENSE).
