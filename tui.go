package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ramilamparo/hid2xbox/joystick"

	tea "github.com/charmbracelet/bubbletea"
)

// --- TUI screens ---

type tuiScreen int

const (
	screenPrereq tuiScreen = iota
	screenSelect
	screenDiscover
	screenMap
	screenReview
)

// --- Xbox output targets ---

var axisTargets = []string{
	"left_trigger", "right_trigger",
	"left_stick_x", "left_stick_y", "right_stick_x", "right_stick_y",
}

var buttonTargets = []string{
	"a", "b", "x", "y",
	"left_shoulder", "right_shoulder",
	"start", "back",
	"left_thumb", "right_thumb",
	"dpad_up", "dpad_down", "dpad_left", "dpad_right",
}

// --- Messages ---

type tickMsg struct {
	axes    []uint32
	buttons uint32
}

// --- Model ---

type model struct {
	screen     tuiScreen
	prereq     prereqStatus

	// Prereq screen state
	downloading  bool
	downloadDone bool
	downloadErr  string

	joysticks  []joystick.Info
	cursor     int
	err        error

	// Discover
	dev            *joystick.Device
	axes           []uint32
	buttons        uint32
	selectedIndex  int
	selectedIsAxis bool

	// Map
	curMapping   Mapping
	targetCursor int

	// Review
	mappings   []Mapping
	configPath string
	saved      bool
	quitting   bool
}

func runTUI(configPath string) error {
	sticks := joystick.Enumerate()
	if len(sticks) == 0 {
		return fmt.Errorf("no joysticks detected — plug in your controller and try again")
	}

	prereq := detectPrereqs()
	startScreen := screenSelect
	if !prereq.ViGEmBusOK || !prereq.ViGEmClientOK {
		startScreen = screenPrereq
	}

	m := &model{
		screen:     startScreen,
		prereq:     prereq,
		joysticks:  sticks,
		configPath: configPath,
	}

	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		return err
	}
	if m.err != nil {
		return m.err
	}
	return nil
}

// --- Init / Update / View ---

func (m *model) Init() tea.Cmd { return nil }

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)
	case prereqMsg:
		m.downloading = false
		m.downloadDone = true
		if msg.err != "" {
			m.downloadErr = msg.err
		} else {
			m.prereq.ViGEmClientOK = true
			m.prereq.ViGEmClientPath = filepath.Join(filepath.Dir(m.prereq.ViGEmClientPath), vigemClientDLL)
		}
		return m, nil
	case tickMsg:
		if m.screen == screenDiscover {
			m.axes = msg.axes
			m.buttons = msg.buttons
		}
		return m, tickCmd(m.dev)
	}
	return m, nil
}

func (m *model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "esc":
		return m.handleBack()
	}

	switch m.screen {
	case screenPrereq:
		return m.prereqUpdate(key)
	case screenSelect:
		return m.selectUpdate(key)
	case screenDiscover:
		return m.discoverUpdate(key)
	case screenMap:
		return m.mapUpdate(key)
	case screenReview:
		return m.reviewUpdate(key)
	}
	return m, nil
}

func (m *model) handleBack() (tea.Model, tea.Cmd) {
	switch m.screen {
	case screenSelect, screenPrereq:
		m.quitting = true
		return m, tea.Quit
	case screenDiscover:
		m.cleanupDevice()
		m.screen = screenSelect
		return m, nil
	case screenMap:
		m.screen = screenDiscover
		return m, tickCmd(m.dev)
	case screenReview:
		m.screen = screenDiscover
		return m, tickCmd(m.dev)
	}
	return m, nil
}

func (m *model) View() string {
	switch m.screen {
	case screenPrereq:
		return m.prereqView()
	case screenSelect:
		return m.selectView()
	case screenDiscover:
		return m.discoverView()
	case screenMap:
		return m.mapView()
	case screenReview:
		return m.reviewView()
	}
	return ""
}

// --- Prereq screen ---

func (m *model) prereqUpdate(key string) (tea.Model, tea.Cmd) {
	if m.downloading {
		return m, nil
	}
	switch key {
	case "d":
		if !m.prereq.ViGEmClientOK {
			m.downloading = true
			return m, func() tea.Msg {
				exe, _ := os.Executable()
				dir := filepath.Dir(exe)
				_, err := downloadViGEmClient(dir)
				if err != nil {
					return prereqMsg{err: err.Error()}
				}
				return prereqMsg{ok: true}
			}
		}
	case "q":
		m.quitting = true
		return m, tea.Quit
	case "c", "enter":
		m.screen = screenSelect
		return m, nil
	}
	return m, nil
}

type prereqMsg struct {
	ok  bool
	err string
}

func (m *model) prereqView() string {
	var b strings.Builder
	b.WriteString("\n  Prerequisites\n\n")

	if m.prereq.ViGEmBusOK {
		b.WriteString("  [OK]  ViGEmBus driver\n")
	} else {
		b.WriteString("  [!!]  ViGEmBus driver — not installed\n")
		b.WriteString("        Download: https://github.com/nefarius/ViGEmBus/releases\n")
	}

	if m.prereq.HidHideOK {
		b.WriteString("  [OK]  HidHide driver\n")
	} else {
		b.WriteString("  [--]  HidHide driver — recommended (prevents double-input)\n")
		b.WriteString("        Download: https://github.com/nefarius/HidHide/releases\n")
	}

	if m.prereq.ViGEmClientOK {
		b.WriteString(fmt.Sprintf("  [OK]  ViGEmClient.dll (%s)\n", m.prereq.ViGEmClientPath))
	} else {
	if m.prereq.ViGEmClientOK {
		b.WriteString(fmt.Sprintf("  [OK]  ViGEmClient.dll (%s)\n", m.prereq.ViGEmClientPath))
	} else if m.downloadDone {
		if m.downloadErr != "" {
			b.WriteString(fmt.Sprintf("  [!!]  ViGEmClient.dll — download failed: %s\n", m.downloadErr))
		} else {
			b.WriteString(fmt.Sprintf("  [OK]  ViGEmClient.dll — downloaded to %s\n", m.prereq.ViGEmClientPath))
		}
	} else if m.downloading {
		b.WriteString("  [..]  ViGEmClient.dll — downloading...\n")
	} else {
		b.WriteString("  [!!]  ViGEmClient.dll — not found\n")
		b.WriteString(fmt.Sprintf("        Expected: %s\n", m.prereq.ViGEmClientPath))
	}
	}

	b.WriteString("\n")
	if m.downloading {
		b.WriteString("  Downloading, please wait...\n")
	} else if m.prereq.ViGEmBusOK && m.prereq.ViGEmClientOK {
		b.WriteString("  [enter] continue  [q] quit\n")
	} else {
		if !m.prereq.ViGEmClientOK {
			b.WriteString("  [d] download ViGEmClient.dll  ")
		}
		b.WriteString("  [c] continue anyway  [q] quit\n")
	}
	return b.String()
}

// --- Select screen ---

func (m *model) selectUpdate(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.joysticks)-1 {
			m.cursor++
		}
	case "enter":
		info := m.joysticks[m.cursor]
		dev, err := joystick.Open(info.ID)
		if err != nil {
			m.err = err
			return m, tea.Quit
		}
		m.dev = dev
		m.screen = screenDiscover
		m.selectedIndex = 0
		m.selectedIsAxis = true
		m.axes = make([]uint32, info.AxisCount)
		return m, tickCmd(dev)
	case "q":
		m.quitting = true
		return m, tea.Quit
	}
	return m, nil
}

func (m *model) selectView() string {
	var b strings.Builder
	b.WriteString("\n  Select Controller\n\n")
	for i, js := range m.joysticks {
		cursor := " "
		if i == m.cursor {
			cursor = ">"
		}
		b.WriteString(fmt.Sprintf(" %s %-32s axes: %d  buttons: %d  VID=%04X PID=%04X\n",
			cursor, js.Name, js.AxisCount, js.ButtonCount, js.VID, js.PID))
	}
	b.WriteString("\n  [enter] select  [q] quit\n")
	return b.String()
}

// --- Discover screen ---

func (m *model) discoverUpdate(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "up", "k":
		if m.selectedIndex > 0 {
			m.selectedIndex--
		}
	case "down", "j":
		max := m.dev.Info().AxisCount + m.dev.Info().ButtonCount - 1
		if m.selectedIndex < max {
			m.selectedIndex++
		}
	case "left", "h", "right", "l":
		m.selectedIsAxis = !m.selectedIsAxis
		if m.selectedIsAxis && m.selectedIndex >= m.dev.Info().AxisCount {
			m.selectedIndex = m.dev.Info().AxisCount - 1
		}
		if !m.selectedIsAxis && m.selectedIndex >= m.dev.Info().ButtonCount {
			m.selectedIndex = m.dev.Info().ButtonCount - 1
		}
	case "enter", "m":
		m.curMapping = Mapping{Source: m.selectedIndex, Deadzone: 0.05}
		if m.selectedIsAxis {
			m.curMapping.Type = "axis"
			r := m.dev.Info().AxisRanges[m.selectedIndex]
			m.curMapping.Min = int(r.Min)
			m.curMapping.Max = int(r.Max)
		} else {
			m.curMapping.Type = "button"
		}
		m.targetCursor = 0
		m.screen = screenMap
		return m, nil
	}
	return m, nil
}

func (m *model) discoverView() string {
	info := m.dev.Info()
	var b strings.Builder
	b.WriteString(fmt.Sprintf("\n  Discover Inputs — %s\n\n", info.Name))

	b.WriteString("  Axes:\n")
	for i := range info.AxisCount {
		marker := " "
		if m.selectedIsAxis && i == m.selectedIndex {
			marker = ">"
		}
		bar := bar(m.axes[i], info.AxisRanges[i].Min, info.AxisRanges[i].Max, 30)
		b.WriteString(fmt.Sprintf(" %s %s  Axis %d  (%d)\n", marker, bar, i, m.axes[i]))
	}

	b.WriteString("\n  Buttons:\n")
	for i := range info.ButtonCount {
		marker := " "
		if !m.selectedIsAxis && i == m.selectedIndex {
			marker = ">"
		}
		state := "○"
		if m.buttons&(1<<i) != 0 {
			state = "●"
		}
		b.WriteString(fmt.Sprintf(" %s %s  Button %d\n", marker, state, i))
	}

	b.WriteString("\n  [↑↓] navigate  [←→] axis/button  [enter] map  [esc] back\n")
	return b.String()
}

// --- Map screen ---

func (m *model) mapUpdate(key string) (tea.Model, tea.Cmd) {
	targets := axisTargets
	if m.curMapping.Type == "button" {
		targets = buttonTargets
	}

	switch key {
	case "up", "k":
		if m.targetCursor > 0 {
			m.targetCursor--
		}
	case "down", "j":
		if m.targetCursor < len(targets)-1 {
			m.targetCursor++
		}
	case "tab":
		m.curMapping.Invert = !m.curMapping.Invert
	case "enter":
		m.curMapping.Target = targets[m.targetCursor]
		m.mappings = append(m.mappings, m.curMapping)
		m.screen = screenReview
		return m, nil
	}
	return m, nil
}

func (m *model) mapView() string {
	targets := axisTargets
	if m.curMapping.Type == "button" {
		targets = buttonTargets
	}

	var b strings.Builder
	inputLabel := fmt.Sprintf("Axis %d", m.curMapping.Source)
	if m.curMapping.Type == "button" {
		inputLabel = fmt.Sprintf("Button %d", m.curMapping.Source)
	}

	b.WriteString(fmt.Sprintf("\n  Map %s\n\n", inputLabel))
	if m.curMapping.Type == "axis" {
		b.WriteString(fmt.Sprintf("  Input range: %d - %d\n", m.curMapping.Min, m.curMapping.Max))
		b.WriteString(fmt.Sprintf("  Invert: %v\n\n", m.curMapping.Invert))
	}
	b.WriteString("  Target:\n")
	for i, t := range targets {
		cursor := " "
		if i == m.targetCursor {
			cursor = ">"
		}
		b.WriteString(fmt.Sprintf(" %s %s\n", cursor, t))
	}

	b.WriteString("\n  [enter] confirm  [tab] toggle invert  [esc] back\n")
	return b.String()
}

// --- Review screen ---

func (m *model) reviewUpdate(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "s":
		cfg := &Config{Name: m.dev.Info().Name, Mappings: m.mappings}
		if err := SaveConfig(m.configPath, cfg); err != nil {
			m.err = err
			return m, tea.Quit
		}
		m.saved = true
		return m, tea.Quit
	case "a":
		m.screen = screenDiscover
		m.selectedIndex = 0
		m.selectedIsAxis = true
		return m, tickCmd(m.dev)
	case "q":
		m.quitting = true
		return m, tea.Quit
	}
	return m, nil
}

func (m *model) reviewView() string {
	info := m.dev.Info()
	var b strings.Builder
	b.WriteString(fmt.Sprintf("\n  Review & Save — %s\n\n", info.Name))
	b.WriteString(fmt.Sprintf("  %-20s → %-20s\n", "Source", "Target"))
	b.WriteString("  " + strings.Repeat("─", 42) + "\n")
	for _, mp := range m.mappings {
		src := fmt.Sprintf("%s %d", mp.Type, mp.Source)
		b.WriteString(fmt.Sprintf("  %-20s → %-20s", src, mp.Target))
		if mp.Type == "axis" {
			b.WriteString(fmt.Sprintf("  %d-%d", mp.Min, mp.Max))
			if mp.Invert {
				b.WriteString(" inv")
			}
		}
		b.WriteString("\n")
	}

	b.WriteString(fmt.Sprintf("\n  Save to: %s\n", m.configPath))
	b.WriteString("\n  [s] save & exit  [a] add mapping  [q] discard\n")
	return b.String()
}

// --- Helpers ---

func tickCmd(dev *joystick.Device) tea.Cmd {
	return tea.Tick(50*time.Millisecond, func(t time.Time) tea.Msg {
		axes, buttons, err := dev.ReadRaw()
		if err != nil {
			return tickMsg{}
		}
		return tickMsg{axes: axes, buttons: buttons}
	})
}

func bar(value, min, max uint32, width int) string {
	if max == min {
		return strings.Repeat("░", width)
	}
	fill := int(float64(value-min) / float64(max-min) * float64(width))
	if fill < 0 {
		fill = 0
	}
	if fill > width {
		fill = width
	}
	return strings.Repeat("█", fill) + strings.Repeat("░", width-fill)
}

func (m *model) cleanupDevice() {
	if m.dev != nil {
		m.dev.Close()
		m.dev = nil
	}
}
