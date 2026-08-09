// Package joystick provides Windows joystick access via the legacy joyGetPosEx API.
// Vendored and modified from github.com/0xcafed00d/joystick — changes:
//   - Raw axis values (not normalized)
//   - VID/PID exposed via Info
//   - Enumerate() to discover all attached joysticks
package joystick

// AxisRange holds the hardware min/max for a single axis.
type AxisRange struct {
	Min, Max uint32
}

// Info describes a joystick discovered via Enumerate.
type Info struct {
	ID         int
	Name       string
	VID, PID   uint16
	AxisCount  int
	ButtonCount int
	AxisRanges []AxisRange // per-axis, length == AxisCount
}

// Device is an open joystick handle.
type Device struct {
	info Info
}

// Enumerate scans joystick IDs 0–15 and returns info for each attached device.
func Enumerate() []Info { return enumerate() }

// Open acquires a joystick by numeric ID (as returned by Enumerate).
func Open(id int) (*Device, error) { return open(id) }

// Info returns a copy of the device metadata.
func (d *Device) Info() Info { return d.info }

// ReadRaw polls the device. Returns raw axis values (native hardware range)
// and a button bitmask. Bits 0..N correspond to buttons 1..N+1.
func (d *Device) ReadRaw() (axes []uint32, buttons uint32, err error) {
	return d.readRaw()
}

// Close releases the device. On Windows, this is a no-op for the legacy API.
func (d *Device) Close() {}
