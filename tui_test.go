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
		curMapping: Mapping{Type: "axis", Source: 2, Target: "right_trigger", Min: 0, Max: 512, Invert: true},
	}
	m.upsertMapping()
	if len(m.mappings) != 2 {
		t.Fatalf("got %d mappings, want 2 (replaced, not appended)", len(m.mappings))
	}
	mp := m.mappings[0]
	if mp.Target != "right_trigger" || mp.Max != 512 || !mp.Invert {
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
