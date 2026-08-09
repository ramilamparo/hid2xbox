//go:build windows

package joystick

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const (
	maxPNameLen  = 32
	maxAxis      = 6
	joyReturnAll          = 0xFF
	digcfDeviceInterface  = 0x00000010
	digcfPresent = 0x00000002
)

var guidHID = windows.GUID{
	Data1: 0x4D1E55B2,
	Data2: 0xF16F,
	Data3: 0x11CF,
	Data4: [8]byte{0x88, 0xCB, 0x00, 0x11, 0x11, 0x00, 0x00, 0x30},
}

type joyCaps struct {
	wMid        uint16
	wPid        uint16
	szPname     [maxPNameLen]uint16
	wXmin       uint32
	wXmax       uint32
	wYmin       uint32
	wYmax       uint32
	wZmin       uint32
	wZmax       uint32
	wNumButtons uint32
	wPeriodMin  uint32
	wPeriodMax  uint32
	wRmin       uint32
	wRmax       uint32
	wUmin       uint32
	wUmax       uint32
	wVmin       uint32
	wVmax       uint32
	wCaps       uint32
	wMaxAxes    uint32
	wNumAxes    uint32
	wMaxButtons uint32
	szRegKey    [maxPNameLen]uint16
	szOEMVxD    [260]uint16
}

type joyInfoEx struct {
	dwSize         uint32
	dwFlags        uint32
	dwAxis         [maxAxis]uint32
	dwButtons      uint32
	dwButtonNumber uint32
	dwPOV          uint32
	dwReserved1    uint32
	dwReserved2    uint32
}

type spDeviceInterfaceData struct {
	cbSize             uint32
	interfaceClassGuid windows.GUID
	flags              uint32
	reserved           uintptr
}

type spDeviceInterfaceDetailData struct {
	cbSize     uint32
	devicePath [1]uint16
}

var (
	winmm           = windows.MustLoadDLL("Winmm.dll")
	procJoyGetPosEx = winmm.MustFindProc("joyGetPosEx")
	procJoyGetCaps  = winmm.MustFindProc("joyGetDevCapsW")

	setupapi                            = windows.MustLoadDLL("setupapi.dll")
	procSetupDiGetClassDevs             = setupapi.MustFindProc("SetupDiGetClassDevsW")
	procSetupDiEnumDeviceInterfaces     = setupapi.MustFindProc("SetupDiEnumDeviceInterfaces")
	procSetupDiGetDeviceInterfaceDetail = setupapi.MustFindProc("SetupDiGetDeviceInterfaceDetailW")
	procSetupDiDestroyDeviceInfoList    = setupapi.MustFindProc("SetupDiDestroyDeviceInfoList")

	kernel32            = windows.MustLoadDLL("kernel32.dll")
	procCreateFile      = kernel32.MustFindProc("CreateFileW")
	procCloseHandle     = kernel32.MustFindProc("CloseHandle")

	hid                      = windows.MustLoadDLL("hid.dll")
	procHidDGetProductString = hid.MustFindProc("HidD_GetProductString")
)

const invalidHandle = ^uintptr(0)

// --- Name resolution ---

func resolveName(caps joyCaps) string {
	if s := hidProductName(caps.wMid, caps.wPid); s != "" {
		return s
	}
	regKey := windows.UTF16ToString(caps.szRegKey[:])
	if regKey != "" {
		if s := oemName(regKey); s != "" {
			return s
		}
	}
	return windows.UTF16ToString(caps.szPname[:])
}

func oemName(regKey string) string {
	const base = `SYSTEM\CurrentControlSet\Control\MediaProperties\PrivateProperties\Joystick\OEM\`
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, base+regKey, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer k.Close()
	name, _, err := k.GetStringValue("OEMName")
	if err != nil {
		return ""
	}
	return name
}

func hidProductName(vid, pid uint16) string {
	devInfo, _, _ := procSetupDiGetClassDevs.Call(
		uintptr(unsafe.Pointer(&guidHID)),
		0, 0,
		uintptr(digcfPresent|digcfDeviceInterface),
	)
	if devInfo == invalidHandle {
		return ""
	}
	defer procSetupDiDestroyDeviceInfoList.Call(devInfo)

	var iface spDeviceInterfaceData
	iface.cbSize = uint32(unsafe.Sizeof(iface))
	var requiredSize uint32

	for i := range 256 {
		ret, _, _ := procSetupDiEnumDeviceInterfaces.Call(
			devInfo, 0, uintptr(unsafe.Pointer(&guidHID)),
			uintptr(i), uintptr(unsafe.Pointer(&iface)),
		)
		if ret == 0 {
			break
		}

		procSetupDiGetDeviceInterfaceDetail.Call(
			devInfo, uintptr(unsafe.Pointer(&iface)), 0, 0,
			uintptr(unsafe.Pointer(&requiredSize)), 0,
		)
		if requiredSize == 0 {
			continue
		}

		buf := make([]byte, requiredSize)
		detail := (*spDeviceInterfaceDetailData)(unsafe.Pointer(&buf[0]))
		detail.cbSize = uint32(unsafe.Sizeof(spDeviceInterfaceDetailData{}))

		ret, _, _ = procSetupDiGetDeviceInterfaceDetail.Call(
			devInfo, uintptr(unsafe.Pointer(&iface)),
			uintptr(unsafe.Pointer(detail)), uintptr(requiredSize),
			0, 0,
		)
		if ret == 0 {
			continue
		}

		path := windows.UTF16PtrToString(&detail.devicePath[0])
		if !pathMatchesVIDPID(path, vid, pid) {
			continue
		}

		if name := readProductString(path); name != "" {
			return name
		}
	}
	return ""
}

func pathMatchesVIDPID(path string, vid, pid uint16) bool {
	needle := fmt.Sprintf("vid_%04x&pid_%04x", vid, pid)
	needleUpper := fmt.Sprintf("VID_%04X&PID_%04X", vid, pid)
	for i := range len(path) - len(needle) + 1 {
		sub := path[i : i+len(needle)]
		if sub == needle || sub == needleUpper {
			return true
		}
	}
	return false
}

func readProductString(devicePath string) string {
	pathPtr, err := windows.UTF16PtrFromString(devicePath)
	if err != nil {
		return ""
	}
	handle, _, _ := procCreateFile.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		0,
		3, // FILE_SHARE_READ | FILE_SHARE_WRITE
		0,
		3, // OPEN_EXISTING
		0, 0,
	)
	if handle == invalidHandle {
		return ""
	}
	defer procCloseHandle.Call(handle)

	var buf [128]uint16
	ret, _, _ := procHidDGetProductString.Call(
		handle,
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
	)
	if ret == 0 {
		return ""
	}
	return windows.UTF16ToString(buf[:])
}

// --- Joystick I/O ---

func buildInfo(id int, caps joyCaps) Info {
	axisCount := int(caps.wNumAxes)
	ranges := make([]AxisRange, axisCount)
	rawRanges := [][2]uint32{
		{caps.wXmin, caps.wXmax},
		{caps.wYmin, caps.wYmax},
		{caps.wZmin, caps.wZmax},
		{caps.wRmin, caps.wRmax},
		{caps.wUmin, caps.wUmax},
		{caps.wVmin, caps.wVmax},
	}
	for i := range axisCount {
		ranges[i] = AxisRange{Min: rawRanges[i][0], Max: rawRanges[i][1]}
	}
	return Info{
		ID:          id,
		Name:        resolveName(caps),
		VID:         caps.wMid,
		PID:         caps.wPid,
		AxisCount:   axisCount,
		ButtonCount: int(caps.wNumButtons),
		AxisRanges:  ranges,
	}
}

func enumerate() []Info {
	var result []Info
	for id := range 16 {
		var caps joyCaps
		ret, _, _ := procJoyGetCaps.Call(
			uintptr(id),
			uintptr(unsafe.Pointer(&caps)),
			unsafe.Sizeof(caps),
		)
		if ret != 0 {
			continue
		}
		result = append(result, buildInfo(id, caps))
	}
	return result
}

func open(id int) (*Device, error) {
	var caps joyCaps
	ret, _, _ := procJoyGetCaps.Call(
		uintptr(id),
		uintptr(unsafe.Pointer(&caps)),
		unsafe.Sizeof(caps),
	)
	if ret != 0 {
		return nil, fmt.Errorf("joystick %d: joyGetDevCaps failed (code %d)", id, ret)
	}
	return &Device{info: buildInfo(id, caps)}, nil
}

func (d *Device) readRaw() ([]uint32, uint32, error) {
	var info joyInfoEx
	info.dwSize = uint32(unsafe.Sizeof(info))
	info.dwFlags = joyReturnAll

	ret, _, _ := procJoyGetPosEx.Call(
		uintptr(d.info.ID),
		uintptr(unsafe.Pointer(&info)),
	)
	if ret != 0 {
		return nil, 0, fmt.Errorf("joystick %d: joyGetPosEx failed (code %d)", d.info.ID, ret)
	}

	axes := make([]uint32, d.info.AxisCount)
	copy(axes, info.dwAxis[:d.info.AxisCount])
	return axes, info.dwButtons, nil
}
