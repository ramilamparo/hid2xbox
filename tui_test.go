package main

import (
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

	for range len(axisTargets) - 2 {
		_, _ = m.mapUpdate("down")
	}
	last := len(axisTargets) - 1
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
