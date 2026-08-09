package main

import (
	"strings"
	"testing"

	"github.com/ramilamparo/hid2xbox/joystick"

	tea "github.com/charmbracelet/bubbletea"
)

func TestTUI_SelectScreen_Navigation(t *testing.T) {
	m := &model{
		screen: screenSelect,
		joysticks: []joystick.Info{
			{ID: 0, Name: "Device A", AxisCount: 4, ButtonCount: 2},
			{ID: 1, Name: "Device B", AxisCount: 6, ButtonCount: 12},
		},
	}

	if m.cursor != 0 {
		t.Fatalf("initial cursor = %d, want 0", m.cursor)
	}

	_, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyDown})
	if m.cursor != 1 {
		t.Errorf("after down: cursor = %d, want 1", m.cursor)
	}

	_, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyDown})
	if m.cursor != 1 {
		t.Errorf("after second down: cursor = %d, want 1 (stays at bottom)", m.cursor)
	}

	_, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyUp})
	if m.cursor != 0 {
		t.Errorf("after up: cursor = %d, want 0", m.cursor)
	}
}

func TestTUI_SelectScreen_Enter(t *testing.T) {
	sticks := joystick.Enumerate()
	if len(sticks) == 0 {
		t.Skip("no joysticks available")
	}
	m := &model{
		screen:    screenSelect,
		joysticks: sticks,
	}

	_, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	defer m.cleanupDevice()

	if m.screen != screenDiscover {
		t.Errorf("after enter: screen = %d, want screenDiscover", m.screen)
	}
	if m.dev == nil {
		t.Fatal("device should be opened after enter")
	}
}

func TestTUI_SelectScreen_Quit(t *testing.T) {
	m := &model{
		screen:    screenSelect,
		joysticks: []joystick.Info{{ID: 0, Name: "X"}},
	}
	_, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlC})
	if !m.quitting {
		t.Error("ctrl+c should set quitting flag on select screen")
	}
}

func TestTUI_DiscoverScreen_EscapeGoesBack(t *testing.T) {
	m := &model{
		screen:        screenDiscover,
		selectedIndex:  0,
		selectedIsAxis: true,
	}
	_, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyEscape})
	if m.screen != screenSelect {
		t.Errorf("esc from discover: screen = %d, want screenSelect", m.screen)
	}
}

func TestTUI_DiscoverScreen_NavigateAxes(t *testing.T) {
	sticks := joystick.Enumerate()
	if len(sticks) == 0 {
		t.Skip("no joysticks available")
	}
	info := sticks[0]
	dev, err := joystick.Open(info.ID)
	if err != nil {
		t.Skipf("cannot open joystick %d: %v", info.ID, err)
	}
	defer dev.Close()

	m := &model{
		screen:        screenDiscover,
		selectedIndex:  0,
		selectedIsAxis: true,
		dev:            dev,
	}

	_, _ = m.discoverUpdate("down")
	if m.selectedIndex != 1 {
		t.Errorf("down: selectedIndex = %d, want 1", m.selectedIndex)
	}

	_, _ = m.discoverUpdate("down")
	if m.selectedIndex != 2 {
		t.Errorf("down: selectedIndex = %d, want 2", m.selectedIndex)
	}

	_, _ = m.discoverUpdate("up")
	if m.selectedIndex != 1 {
		t.Errorf("up: selectedIndex = %d, want 1", m.selectedIndex)
	}
}

func TestTUI_MapScreen_TargetNavigation(t *testing.T) {
	m := &model{
		screen: screenMap,
		curMapping: Mapping{
			Type:   "axis",
			Source: 2,
			Min:    0,
			Max:    1023,
		},
		targetCursor: 0,
	}

	_, _ = m.mapUpdate("down")
	if m.targetCursor != 1 {
		t.Errorf("targetCursor = %d, want 1", m.targetCursor)
	}

	allTargets := len(axisTargets) + len(buttonTargets) // axis type shows combined
	for range allTargets - 2 {
		_, _ = m.mapUpdate("down")
	}
	last := allTargets - 1
	if m.targetCursor != last {
		t.Errorf("targetCursor at bottom = %d, want %d", m.targetCursor, last)
	}

	_, _ = m.mapUpdate("down")
	if m.targetCursor != last {
		t.Errorf("targetCursor past bottom = %d, want %d", m.targetCursor, last)
	}
}

func TestTUI_MapScreen_EnterAddsMapping(t *testing.T) {
	m := &model{
		screen: screenMap,
		curMapping: Mapping{
			Type:   "axis",
			Source: 2,
			Min:    0,
			Max:    1023,
		},
		targetCursor: 0,
	}

	_, _ = m.mapUpdate("enter")

	if m.screen != screenReview {
		t.Errorf("after enter: screen = %d, want screenReview", m.screen)
	}
	if len(m.mappings) != 1 {
		t.Fatalf("mappings len = %d, want 1", len(m.mappings))
	}
	mp := m.mappings[0]
	if mp.Target != "left_trigger" {
		t.Errorf("mapping target = %q, want left_trigger", mp.Target)
	}
}

func TestTUI_ReviewScreen_AddMappingReturns(t *testing.T) {
	m := &model{
		screen:        screenReview,
		mappings:      []Mapping{{Type: "button", Source: 0, Target: "a"}},
		selectedIndex:  0,
		selectedIsAxis: true,
	}

	_, _ = m.reviewUpdate("a")
	if m.screen != screenDiscover {
		t.Errorf("after 'a': screen = %d, want screenDiscover", m.screen)
	}
}

func TestTUI_ReviewScreen_Quit(t *testing.T) {
	m := &model{
		screen:   screenReview,
		mappings: []Mapping{{Type: "button", Source: 0, Target: "a"}},
	}

	_, _ = m.reviewUpdate("q")
	if !m.quitting {
		t.Error("'q' on review should set quitting")
	}
}

func TestTUI_ReviewScreen_Save(t *testing.T) {
	sticks := joystick.Enumerate()
	if len(sticks) == 0 {
		t.Skip("no joysticks available")
	}
	dev, err := joystick.Open(sticks[0].ID)
	if err != nil {
		t.Skipf("cannot open joystick: %v", err)
	}
	defer dev.Close()

	dir := t.TempDir()
	path := dir + "/test-config.json"

	m := &model{
		screen:     screenReview,
		dev:        dev,
		mappings:   []Mapping{{Type: "button", Source: 0, Target: "a"}},
		configPath: path,
	}

	_, _ = m.reviewUpdate("s")
	if !m.saved {
		t.Error("'s' should set saved flag")
	}
}

func TestUpsertMapping_Append(t *testing.T) {
	m := &model{
		mappings:    []Mapping{{Type: "button", Source: 0, Target: "a"}},
		curMapping:  Mapping{Type: "axis", Source: 1, Target: "right_trigger"},
	}
	m.upsertMapping()
	if len(m.mappings) != 2 {
		t.Fatalf("got %d mappings, want 2 (appended)", len(m.mappings))
	}
	if m.mappings[1].Target != "right_trigger" {
		t.Errorf("second mapping target = %q, want right_trigger", m.mappings[1].Target)
	}
}

func TestUpsertMapping_Replace(t *testing.T) {
	m := &model{
		mappings: []Mapping{
			{Type: "axis", Source: 2, Target: "left_trigger", Min: 0, Max: 1023},
			{Type: "button", Source: 0, Target: "a"},
		},
		curMapping: Mapping{Type: "axis", Source: 2, Target: "left_trigger", Min: 0, Max: 512, Invert: true},
	}
	m.upsertMapping()
	if len(m.mappings) != 2 {
		t.Fatalf("got %d mappings, want 2 (replaced, not appended)", len(m.mappings))
	}
	mp := m.mappings[0]
	if mp.Target != "left_trigger" || mp.Max != 512 || !mp.Invert {
		t.Errorf("mapping not replaced: %+v", mp)
	}
	if m.mappings[1].Target != "a" {
		t.Error("second mapping should be unchanged")
	}
}

func TestUpsertMapping_TypeMismatch(t *testing.T) {
	m := &model{
		mappings:   []Mapping{{Type: "axis", Source: 1, Target: "left_trigger"}},
		curMapping: Mapping{Type: "button", Source: 1, Target: "a"},
	}
	m.upsertMapping()
	if len(m.mappings) != 2 {
		t.Fatalf("got %d mappings, want 2 (different types = no conflict)", len(m.mappings))
	}
}

func TestUpsertMapping_DifferentTargetNoConflict(t *testing.T) {
	m := &model{
		mappings:   []Mapping{{Type: "axis", Source: 0, Target: "left_trigger"}},
		curMapping: Mapping{Type: "axis", Source: 0, Target: "right_trigger"},
	}
	m.upsertMapping()
	if len(m.mappings) != 2 {
		t.Fatalf("got %d mappings, want 2 (different targets = no conflict)", len(m.mappings))
	}
	if m.conflictReplaced {
		t.Error("conflictReplaced should be false for different targets")
	}
}

func TestUpsertMapping_SetsConflictFlag(t *testing.T) {
	m := &model{
		mappings:   []Mapping{{Type: "axis", Source: 0, Target: "left_trigger"}},
		curMapping: Mapping{Type: "axis", Source: 0, Target: "left_trigger", Max: 512},
	}
	m.upsertMapping()
	if !m.conflictReplaced {
		t.Error("conflictReplaced should be true for same (type,source,target)")
	}
	if m.mappings[0].Max != 512 {
		t.Error("mapping should be updated with new values")
	}
}

func TestConfigFlag_Multiple(t *testing.T) {
	var c configsFlag
	if err := c.Set("a.json"); err != nil {
		t.Fatal(err)
	}
	if err := c.Set("b.json"); err != nil {
		t.Fatal(err)
	}
	if len(c) != 2 || c[0] != "a.json" || c[1] != "b.json" {
		t.Errorf("got %v, want [a.json b.json]", c)
	}
	if c.String() != "a.json, b.json" {
		t.Errorf("String() = %q, want %q", c.String(), "a.json, b.json")
	}
}

func TestConfigFlag_Default(t *testing.T) {
	var c configsFlag
	if len(c) != 0 {
		t.Errorf("empty configsFlag should be len 0, got %d", len(c))
	}
}

func TestHelpView_AllScreens(t *testing.T) {
	m := &model{help: true}
	out := m.helpView()
	if !strings.Contains(out, "No keybindings yet") {
		t.Error("empty mappings should show placeholder")
	}
	m = &model{
		help: true,
		mappings: []Mapping{
			{Type: "axis", Source: 2, Target: "right_trigger"},
			{Type: "button", Source: 0, Target: "a"},
		},
	}
	out = m.helpView()
	if !strings.Contains(out, "axis 2") {
		t.Error("missing axis mapping")
	}
	if !strings.Contains(out, "button 0") {
		t.Error("missing button mapping")
	}
	if !strings.Contains(out, "→") {
		t.Error("missing mapping arrow")
	}
}

func TestBuildConfig_MultiDevice(t *testing.T) {
	// Two devices mapped in one session: PXN (wheel) + Arduino (handbrake).
	m := &model{
		mappings: []Mapping{
			{Device: 0, Type: "axis", Source: 0, Target: "right_stick_x", Max: 65535, Mode: "centered", Deadzone: 0.05, Direction: "both"},
			{Device: 1, Type: "axis", Source: 3, Target: "left_trigger", Max: 65535, Mode: "normal", Deadzone: 0.05},
		},
		loadedConfig: &Config{
			Devices: []DeviceSpec{
				{Name: "PXN-V12lite", ID: 4},
				{Name: "Arduino Leonardo", ID: 3},
			},
		},
	}
	cfg := m.buildConfig()
	if len(cfg.Devices) != 2 {
		t.Fatalf("want 2 devices, got %d", len(cfg.Devices))
	}
	if len(cfg.Mappings) != 2 {
		t.Fatalf("want 2 mappings, got %d", len(cfg.Mappings))
	}
	if cfg.Devices[0].Name != "PXN-V12lite" {
		t.Errorf("device[0].Name = %q", cfg.Devices[0].Name)
	}
	if cfg.Devices[1].Name != "Arduino Leonardo" {
		t.Errorf("device[1].Name = %q", cfg.Devices[1].Name)
	}
	if cfg.Mappings[0].Device != 0 {
		t.Errorf("mapping[0].Device = %d, want 0", cfg.Mappings[0].Device)
	}
	if cfg.Mappings[1].Device != 1 {
		t.Errorf("mapping[1].Device = %d, want 1", cfg.Mappings[1].Device)
	}
}

func TestBuildConfig_SingleDevice(t *testing.T) {
	m := &model{
		mappings: []Mapping{
			{Device: 0, Type: "axis", Source: 0, Target: "right_stick_x", Max: 65535, Mode: "centered"},
		},
	}
	cfg := m.buildConfig()
	if len(cfg.Devices) != 1 {
		t.Fatalf("want 1 device, got %d", len(cfg.Devices))
	}
	if len(cfg.Mappings) != 1 {
		t.Fatalf("want 1 mapping, got %d", len(cfg.Mappings))
	}
}

func TestBuildConfig_NewDeviceNotInLoadedConfig(t *testing.T) {
	// loadedConfig has 1 device, but a mapping references device index 1.
	// This simulates: user had PXN config, ran --setup, added Arduino.
	m := &model{
		mappings: []Mapping{
			{Device: 0, Type: "axis", Source: 0, Target: "right_stick_x", Mode: "centered"},
			{Device: 1, Type: "axis", Source: 3, Target: "left_trigger", Mode: "normal"},
		},
		loadedConfig: &Config{
			Devices: []DeviceSpec{{Name: "PXN-V12lite", ID: 4}},
		},
		idxToName: map[int]string{0: "PXN-V12lite", 1: "Arduino Leonardo"},
		idxToID:   map[int]int{0: 4, 1: 3},
		deviceIdx: 0, // user was on PXN when they saved
	}
	cfg := m.buildConfig()
	if len(cfg.Devices) < 2 {
		t.Fatalf("want at least 2 devices, got %d — Arduino lost", len(cfg.Devices))
	}
	if len(cfg.Mappings) != 2 {
		t.Fatalf("want 2 mappings, got %d", len(cfg.Mappings))
	}
	if cfg.Devices[1].Name == "" && cfg.Devices[1].ID == 0 {
		t.Error("device[1] is empty — Arduino device info lost")
	}
}

func TestMultiDeviceWorkflow_SwitchAndSave(t *testing.T) {
	// Full flow: PXN mapped, tab to Arduino, Arduino mapped, save.
	// loadedConfig has PXN only (from a previous --setup run).
	m := &model{
		mappings: []Mapping{
			{Device: 0, Type: "axis", Source: 0, Target: "right_stick_x", Max: 65535, Mode: "centered", Deadzone: 0.05, Direction: "both"},
			{Device: 1, Type: "axis", Source: 3, Target: "left_trigger", Max: 65535, Mode: "normal", Deadzone: 0.05},
		},
		loadedConfig: &Config{
			Devices: []DeviceSpec{{Name: "PXN-V12lite", ID: 4}},
		},
		idxToName: map[int]string{0: "PXN-V12lite", 1: "Arduino Leonardo"},
		idxToID:   map[int]int{0: 4, 1: 3},
		deviceIdx: 1, // currently on Arduino after tab
	}

	cfg := m.buildConfig()

	// Both devices must be present.
	if len(cfg.Devices) != 2 {
		t.Fatalf("want 2 devices, got %d", len(cfg.Devices))
	}
	if cfg.Devices[0].Name != "PXN-V12lite" || cfg.Devices[0].ID != 4 {
		t.Errorf("device[0] = %+v, want PXN-V12lite id=4", cfg.Devices[0])
	}
	if cfg.Devices[1].Name != "Arduino Leonardo" || cfg.Devices[1].ID != 3 {
		t.Errorf("device[1] = %+v, want Arduino Leonardo id=3", cfg.Devices[1])
	}
	if len(cfg.Mappings) != 2 {
		t.Fatalf("want 2 mappings, got %d", len(cfg.Mappings))
	}
	if cfg.Mappings[0].Device != 0 {
		t.Errorf("mapping[0].Device = %d, want 0", cfg.Mappings[0].Device)
	}
	if cfg.Mappings[1].Device != 1 {
		t.Errorf("mapping[1].Device = %d, want 1", cfg.Mappings[1].Device)
	}
}

func TestMultiDeviceWorkflow_OnlyNewDeviceMapped(t *testing.T) {
	// User loaded existing PXN config, switched immediately to Arduino,
	// mapped only Arduino (didn't re-map PXN).
	m := &model{
		mappings: []Mapping{
			// PXN mapping from loaded config (Device defaults to 0).
			{Type: "axis", Source: 0, Target: "right_stick_x", Max: 65535, Mode: "centered"},
			// New Arduino mapping (Device=1).
			{Device: 1, Type: "axis", Source: 3, Target: "left_trigger", Max: 65535, Mode: "normal"},
		},
		loadedConfig: &Config{
			Devices: []DeviceSpec{{Name: "PXN-V12lite", ID: 4}},
			Mappings: []Mapping{
				{Type: "axis", Source: 0, Target: "right_stick_x", Max: 65535, Mode: "centered"},
			},
		},
		idxToName: map[int]string{0: "PXN-V12lite", 1: "Arduino Leonardo"},
		idxToID:   map[int]int{0: 4, 1: 3},
		deviceIdx: 1,
	}

	cfg := m.buildConfig()

	if len(cfg.Devices) != 2 {
		t.Fatalf("want 2 devices, got %d — PXN lost", len(cfg.Devices))
	}
	if cfg.Devices[0].Name != "PXN-V12lite" {
		t.Errorf("device[0] = %+v", cfg.Devices[0])
	}
	if len(cfg.Mappings) != 2 {
		t.Fatalf("want 2 mappings, got %d", len(cfg.Mappings))
	}
}

func TestMultiDevice_TabPreservesMappings(t *testing.T) {
	// Simulate: user is on PXN review screen, presses tab to switch.
	// Mappings should survive the device switch.
	m := &model{
		screen: screenReview,
		mappings: []Mapping{
			{Device: 0, Type: "axis", Source: 0, Target: "right_stick_x", Mode: "centered"},
		},
		idxToName: map[int]string{0: "PXN-V12lite"},
		idxToID:   map[int]int{0: 4},
	}

	_, _ = m.reviewUpdate("tab")

	if m.screen != screenSelect {
		t.Fatalf("after tab: screen=%d, want screenSelect", m.screen)
	}
	if len(m.mappings) != 1 {
		t.Fatalf("mappings lost after tab: got %d, want 1", len(m.mappings))
	}
	if m.mappings[0].Device != 0 || m.mappings[0].Target != "right_stick_x" {
		t.Errorf("mapping corrupted: %+v", m.mappings[0])
	}
}

func TestMultiDevice_EscFromReviewPreservesMappings(t *testing.T) {
	m := &model{
		screen: screenReview,
		mappings: []Mapping{
			{Device: 0, Type: "axis", Source: 0, Target: "right_stick_x", Mode: "centered"},
		},
		idxToName: map[int]string{0: "PXN-V12lite"},
		idxToID:   map[int]int{0: 4},
	}

	// esc from review should go to select (preserving mappings).
	_, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyEsc})

	if m.screen != screenSelect {
		t.Fatalf("after esc: screen=%d, want screenSelect", m.screen)
	}
	if len(m.mappings) != 1 {
		t.Fatalf("mappings lost: got %d, want 1", len(m.mappings))
	}
}

func TestMultiDevice_NoLoadedConfig_UniqueIndices(t *testing.T) {
	// Fresh --setup with no existing config.
	// Arduino selected first (idx 0), PXN selected second (idx 1).
	m := &model{
		screen:    screenSelect,
		joysticks: []joystick.Info{
			{ID: 3, Name: "Arduino Leonardo", AxisCount: 6, ButtonCount: 32},
			{ID: 4, Name: "PXN-V12lite", AxisCount: 6, ButtonCount: 32},
		},
		idxToName: map[int]string{},
		idxToID:   map[int]int{},
	}

	// Select Arduino (simulate the effect without opening device).
	m.deviceIdx = 0
	m.idxToName[0] = "Arduino Leonardo"
	m.idxToID[0] = 3
	// Add Arduino mapping.
	m.mappings = append(m.mappings, Mapping{Device: 0, Type: "axis", Source: 3, Target: "left_trigger"})

	// Tab to switch device.
	m.deviceIdx = len(m.idxToName) // simulate new index assignment
	m.idxToName[1] = "PXN-V12lite"
	m.idxToID[1] = 4
	// Add PXN mapping.
	m.mappings = append(m.mappings, Mapping{Device: 1, Type: "axis", Source: 0, Target: "right_stick_x"})

	// Save.
	cfg := m.buildConfig()

	if len(cfg.Devices) != 2 {
		t.Fatalf("want 2 devices, got %d", len(cfg.Devices))
	}
	if cfg.Devices[0].Name != "Arduino Leonardo" {
		t.Errorf("device[0] = %q, want Arduino Leonardo", cfg.Devices[0].Name)
	}
	if cfg.Devices[1].Name != "PXN-V12lite" {
		t.Errorf("device[1] = %q, want PXN-V12lite", cfg.Devices[1].Name)
	}
	if len(cfg.Mappings) != 2 {
		t.Fatalf("want 2 mappings, got %d", len(cfg.Mappings))
	}
	if cfg.Mappings[0].Device != 0 || cfg.Mappings[1].Device != 1 {
		t.Errorf("mapping devices: %d, %d", cfg.Mappings[0].Device, cfg.Mappings[1].Device)
	}
}
