//go:build !windows

package joystick

import "fmt"

// Stub for non-Windows platforms. The joystick package uses the Windows
// joyGetPosEx API and does not work on Linux/macOS. This stub allows the
// rest of the project to compile and run portable tests in Docker/WSL.

func enumerate() []Info { return nil }

func open(id int) (*Device, error) {
	return nil, fmt.Errorf("joystick: not supported on this platform")
}

func (d *Device) readRaw() ([]uint32, uint32, error) {
	return nil, 0, fmt.Errorf("joystick: not supported on this platform")
}

func (d *Device) Close() {}
