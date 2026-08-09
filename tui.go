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

var thresholds = []float64{0.25, 0.50, 0.75, 0.90}

var directions = []string{"both", "positive", "negative"}

func isStickTarget(t string) bool {
	switch t {
	case "left_stick_x", "left_stick_y", "right_stick_x", "right_stick_y":
		return true
	}
	return false
}

func (m *model) viewMappings() string {
	if len(m.mappings) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n  Keybindings:\n")
	for _, mp := range m.mappings {
		devName := fmt.Sprintf("d%d", mp.Device)
		if n, ok := m.idxToName[mp.Device]; ok && n != "" {
			devName = n
		}
		src := fmt.Sprintf("%s %s %d", devName, mp.Type, mp.Source)
		dir := ""
		if mp.Direction != "" && mp.Direction != "both" {
			dir = fmt.Sprintf(" (%s)", mp.Direction)
		}
		thr := ""
		if mp.Threshold > 0 {
			thr = fmt.Sprintf("  thr=%.2f", mp.Threshold)
		}
		b.WriteString(fmt.Sprintf("  %-10s → %-18s%s%s\n", src, mp.Target, dir, thr))
	}
	return b.String()
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
	selectedID     int
	deviceIdx      int  // index into loadedConfig.Devices for the current device
	idxToName      map[int]string // deviceIdx → joystick name (tracked across session)

	// Map
	curMapping   Mapping
	targetCursor int
	modeIndex    int
	thresholdIndex int
	directionIndex  int
	conflictReplaced bool
	help            bool
	configName      string
	loadedConfig    *Config

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

	// Load existing config so keybindings are visible from the start.
	var mappings []Mapping
	var configName string
	var loadedCfg *Config
	if cfg, err := LoadConfig(configPath); err == nil {
		loadedCfg = cfg
		mappings = cfg.Mappings
		if len(cfg.Devices) > 0 {
			configName = cfg.Devices[0].Name
		}
	}

	startScreen := screenSelect
	if !prereq.ViGEmBusOK || !prereq.ViGEmClientOK {
		startScreen = screenPrereq
	}

	m := &model{
		screen:     startScreen,
		prereq:     prereq,
		joysticks:  sticks,
		configPath: configPath,
		mappings:   mappings,
		configName: configName,
		loadedConfig: loadedCfg,
		idxToName:    map[int]string{},
	}

	// Pre-populate from loaded config so existing device indices are known.
	if loadedCfg != nil {
		for i, ds := range loadedCfg.Devices {
			m.idxToName[i] = ds.Name
		}
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
		if m.dev != nil {
			return m, tickCmd(m.dev)
		}
		}
		return m, nil
	}
	return m, nil
}

func (m *model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "?":
		m.help = !m.help
		return m, nil
	case "esc":
		if m.help {
			m.help = false
			return m, nil
		}
		return m.handleBack()
	}

	if m.help {
		m.help = false
		return m, nil
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
		m.screen = screenSelect
		m.cleanupDevice()
		return m, nil
	}
	return m, nil
}

func (m *model) View() string {
	if m.help {
		return m.helpView()
	}
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
		b.WriteString("  [enter] continue  [q] quit  [?] help\n")
	} else {
		if !m.prereq.ViGEmClientOK {
			b.WriteString("  [d] download ViGEmClient.dll  ")
		}
		b.WriteString("  [c] continue anyway  [q] quit  [?] help\n")
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
		m.selectedID = info.ID
		// Determine device index in the config.
		m.deviceIdx = -1
		if m.loadedConfig != nil {
			for i, ds := range m.loadedConfig.Devices {
				if contains(ds.Name, info.Name) || contains(info.Name, ds.Name) {
					m.deviceIdx = i
					break
				}
			}
		}
		if m.deviceIdx < 0 {
			// New device — use next available index.
			if m.loadedConfig != nil {
				m.deviceIdx = len(m.loadedConfig.Devices)
			} else {
			// Use next unused index based on already-tracked devices.
			m.deviceIdx = len(m.idxToName)
		}
			}
		if m.idxToName == nil {
			m.idxToName = map[int]string{}
		}
		m.idxToName[m.deviceIdx] = info.Name
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
		b.WriteString(fmt.Sprintf(" %s %-28s axes: %d  buttons: %d  VID=%04X PID=%04X\n",
			cursor, js.Name, js.AxisCount, js.ButtonCount, js.VID, js.PID))
	}
	sel := ""
	if m.cursor < len(m.joysticks) {
		sel = m.joysticks[m.cursor].Name + "  —  "
	}
	b.WriteString(fmt.Sprintf("\n  %s[enter] select  [q] quit  [?] help\n", sel))
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
		max := m.dev.Info().AxisCount - 1
		if !m.selectedIsAxis {
			max = m.dev.Info().ButtonCount - 1
		}
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
		if m.selectedIndex < 0 {m.selectedIndex = 0}
	case "enter", "m":
		m.curMapping = Mapping{Source: m.selectedIndex, Deadzone: 0.05}
		m.curMapping.Device = m.deviceIdx
		if m.selectedIsAxis {
			m.curMapping.Type = "axis"
			r := m.dev.Info().AxisRanges[m.selectedIndex]
			m.curMapping.Min = int(r.Min)
			m.curMapping.Max = int(r.Max)
		} else {
			m.curMapping.Type = "button"
		}
		m.targetCursor = 0
		m.modeIndex = modeToIndex(m.curMapping.Mode)
		m.screen = screenMap
		return m, nil
	case "tab":
		m.screen = screenSelect
		m.selectedIndex = m.selectedID
		m.cleanupDevice()
		return m, nil
	}
	return m, nil
}

var axisModes = []struct{ label, desc string }{
	{"normal",            "0 → max  ──→  0 → 1"},
	{"inverted",          "0 → max  ──→  1 → 0"},
	{"centered",          "min←mid→max  ──→  -1←0→+1"},
	{"centered_inverted", "min←mid→max  ──→  +1←0→-1"},
}

func modeToIndex(mode string) int {
	switch mode {
	case "inverted":
		return 1
	case "centered":
		return 2
	case "centered_inverted":
		return 3
	}
	return 0
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

	b.WriteString("\n  [↑↓] navigate  [←→] axis/button  [enter] map  [esc] back  [tab] switch device  [?] help\n")
	b.WriteString(m.viewMappings())
	return b.String()
}

// --- Map screen ---

func (m *model) mapUpdate(key string) (tea.Model, tea.Cmd) {
	targets := axisTargets
	if m.curMapping.Type == "button" {
		targets = buttonTargets
	}
	if m.curMapping.Type == "axis" {
		targets = append(append([]string{}, axisTargets...), buttonTargets...)
	}

	// Clear one-shot conflict message on any key.
	m.conflictReplaced = false

	switch key {
	case "up", "k":
		if m.targetCursor > 0 {
			m.targetCursor--
		}
	case "down", "j":
		if m.targetCursor < len(targets)-1 {
			m.targetCursor++
		}
	case "left", "h":
		if m.modeIndex > 0 {
			m.modeIndex--
		}
	case "right", "l":
		if m.modeIndex < len(axisModes)-1 {
			m.modeIndex++
		}
	case "t":
		if m.curMapping.Type == "axis" && m.targetCursor < len(targets) {
			if _, isBtn := buttonMap[targets[m.targetCursor]]; isBtn {
				m.thresholdIndex = (m.thresholdIndex + 1) % len(thresholds)
			}
		}
	case "d":
		if m.curMapping.Type == "axis" && m.targetCursor < len(targets) {
			if isStickTarget(targets[m.targetCursor]) || isButtonTarget(targets[m.targetCursor]) {
				m.directionIndex = (m.directionIndex + 1) % len(directions)
			}
		}
	case "enter":
		m.curMapping.Target = targets[m.targetCursor]
		m.curMapping.Mode = axisModes[m.modeIndex].label
		if m.curMapping.Type == "axis" {
			_, isBtn := buttonMap[m.curMapping.Target]
			isStick := isStickTarget(m.curMapping.Target)
			if isBtn {
				m.curMapping.Threshold = thresholds[m.thresholdIndex]
				m.curMapping.Direction = directions[m.directionIndex]
			} else if isStick {
				m.curMapping.Threshold = 0
				m.curMapping.Direction = directions[m.directionIndex]
			} else {
				m.curMapping.Threshold = 0
				m.curMapping.Direction = ""
			}
		}
		m.upsertMapping()
		m.screen = screenReview
		return m, nil
	case "tab":
		m.screen = screenSelect
		m.cleanupDevice()
		return m, nil
	}
	return m, nil
}

// upsertMapping replaces an existing mapping for the same (type, source, target) or appends.
func (m *model) upsertMapping() {
	for i, existing := range m.mappings {
		if existing.Type == m.curMapping.Type && existing.Source == m.curMapping.Source && existing.Target == m.curMapping.Target {
			m.mappings[i] = m.curMapping
			m.conflictReplaced = true
			return
		}
	}
	m.mappings = append(m.mappings, m.curMapping)
}

func (m *model) mapView() string {
	targets := axisTargets
	if m.curMapping.Type == "button" {
		targets = buttonTargets
	}
	// For axis mappings, show all targets (axis + button) so the user can map
	// an axis to a button trigger or vice versa.
	if m.curMapping.Type == "axis" {
		targets = append(append([]string{}, axisTargets...), buttonTargets...)
	}

	var b strings.Builder
	inputLabel := fmt.Sprintf("Axis %d", m.curMapping.Source)
	if m.curMapping.Type == "button" {
		inputLabel = fmt.Sprintf("Button %d", m.curMapping.Source)
	}

	b.WriteString(fmt.Sprintf("\n  Map %s\n\n", inputLabel))
	if m.curMapping.Type == "axis" {
		b.WriteString(fmt.Sprintf("  Input range: %d - %d\n", m.curMapping.Min, m.curMapping.Max))
		mode := axisModes[m.modeIndex]
		b.WriteString(fmt.Sprintf("  Mode: %s  (%s)\n", mode.label, mode.desc))

		// Show threshold when target is a button.
		if m.targetCursor < len(targets) {
			sel := targets[m.targetCursor]
			if _, isBtn := buttonMap[sel]; isBtn {
				b.WriteString(fmt.Sprintf("  Threshold: %.2f\n", thresholds[m.thresholdIndex]))
			}
		}

		// Show direction when target is a stick.
		if m.targetCursor < len(targets) {
			sel := targets[m.targetCursor]
			if isStickTarget(sel) || isButtonTarget(sel) {
				b.WriteString(fmt.Sprintf("  Direction: %s\n", directions[m.directionIndex]))
			}
		}

		// Visual preview
		b.WriteString("  " + modeBar(m.modeIndex, 40) + "\n\n")
	}

	if m.conflictReplaced {
		b.WriteString("  [!] Replaced existing mapping\n\n")
	}

	b.WriteString("  Target:\n")
	for i, t := range targets {
		cursor := " "
		if i == m.targetCursor {
			cursor = ">"
		}
		b.WriteString(fmt.Sprintf(" %s %s\n", cursor, t))
	}

	b.WriteString("\n  [←→] mode  [t] threshold  [d] direction  [enter] confirm  [esc] back  [tab] switch device  [?] help\n")
	b.WriteString(m.viewMappings())
	return b.String()
}

// --- Review screen ---

func (m *model) reviewUpdate(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "s":
		cfg := m.buildConfig()
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
	case "tab":
		m.screen = screenSelect
		m.cleanupDevice()
		return m, nil
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

	// Detect shared-source pairs.
	type srcKey struct {
		typ    string
		source int
	}
	seen := map[srcKey]bool{}
	for _, mp := range m.mappings {
		k := srcKey{mp.Type, mp.Source}
		if seen[k] {
			seen[k] = false // mark as duplicate (already printed one)
		}
		if _, ok := seen[k]; !ok {
			seen[k] = true
		}
	}

	for _, mp := range m.mappings {
		k := srcKey{mp.Type, mp.Source}
		marker := " "
		if v, ok := seen[k]; ok && !v {
			marker = "*"
		}
		src := fmt.Sprintf("%s %d", mp.Type, mp.Source)
		b.WriteString(fmt.Sprintf(" %s%-19s → %-20s", marker, src, mp.Target))
		if mp.Type == "axis" {
			b.WriteString(fmt.Sprintf("  %d-%d", mp.Min, mp.Max))
			if mp.Mode != "" && mp.Mode != "normal" {
				b.WriteString(fmt.Sprintf(" %s", mp.Mode))
			}
			if mp.Direction != "" && mp.Direction != "both" {
				b.WriteString(fmt.Sprintf(" dir=%s", mp.Direction))
			}
			if mp.Threshold > 0 {
				b.WriteString(fmt.Sprintf(" thr=%.2f", mp.Threshold))
			}
		}
		b.WriteString("\n")
	}

	b.WriteString(fmt.Sprintf("\n  Save to: %s\n", m.configPath))
	b.WriteString("\n  [s] save & exit  [a] add mapping  [tab] switch device  [esc] device list  [q] discard  [?] help\n")
	return b.String()
}

func (m *model) helpView() string {
	if len(m.mappings) == 0 {
		return "\n  No keybindings yet — map some inputs first.\n\n  Press any key to close\n"
	}
	var b strings.Builder
	name := ""
	if m.dev != nil {
		name = " — " + m.dev.Info().Name
	} else if m.configName != "" {
		name = " — " + m.configName
	}
	b.WriteString(fmt.Sprintf("\n  Keybindings%s\n", name))
	b.WriteString("  " + strings.Repeat("─", 42) + "\n\n")

	for _, mp := range m.mappings {
		devName := fmt.Sprintf("d%d", mp.Device)
		if n, ok := m.idxToName[mp.Device]; ok && n != "" {
			devName = n
		}
		src := fmt.Sprintf("%s %s %d", devName, mp.Type, mp.Source)
		dir := ""
		if mp.Direction != "" && mp.Direction != "both" {
			dir = fmt.Sprintf(" (%s)", mp.Direction)
		}
		thr := ""
		if mp.Threshold > 0 {
			thr = fmt.Sprintf("  thr=%.2f", mp.Threshold)
		}
		b.WriteString(fmt.Sprintf("  %-10s → %-18s%s%s\n", src, mp.Target, dir, thr))
	}
	b.WriteString("\n  Press any key to close\n")
	return b.String()
}

func (m *model) buildConfig() *Config {
	// Collect all unique device infos referenced by mappings.
	type devInfo struct {
		name string
	}
	devMap := map[int]devInfo{}
	for _, mp := range m.mappings {
		if _, ok := devMap[mp.Device]; ok {
			continue
		}
		var name string
		// Prefer the tracked idxToName map (populated during device selection).
		if m.idxToName != nil {
			if n, ok := m.idxToName[mp.Device]; ok {
				name = n
			}
		}
		// Fall back to loaded config.
		if name == "" && m.loadedConfig != nil && mp.Device < len(m.loadedConfig.Devices) {
			ds := m.loadedConfig.Devices[mp.Device]
			name = ds.Name
		}
		// Last resort: current device.
		if name == "" && m.dev != nil && mp.Device == m.deviceIdx {
			name = m.dev.Info().Name
		}
		devMap[mp.Device] = devInfo{name: name}
	}

	// Build device list covering all indices up to the max.
	maxIdx := 0
	for idx := range devMap {
		if idx > maxIdx {
			maxIdx = idx
		}
	}
	devices := make([]DeviceSpec, maxIdx+1)
	for idx := range devices {
		if di, ok := devMap[idx]; ok {
			devices[idx] = DeviceSpec{Name: di.name}
		}
	}

	return &Config{
		Devices:  devices,
		Mappings: m.mappings,
	}
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

func modeBar(modeIndex, width int) string {
	half := width / 2
	switch modeIndex {
	case 0: // normal: fills left to right
		return strings.Repeat("░", width) + " → " + strings.Repeat("█", width)
	case 1: // inverted: fills right to left
		return strings.Repeat("█", width) + " → " + strings.Repeat("░", width)
	case 2: // centered: center to both ends
		return strings.Repeat("█", half) + "|░|" + strings.Repeat("█", half) + " → " + strings.Repeat("█", half) + "░" + strings.Repeat("█", half)
	case 3: // centered_inverted: both ends to center
		return strings.Repeat("█", half) + "|░|" + strings.Repeat("█", half) + " → " + strings.Repeat("█", half) + "░" + strings.Repeat("█", half)
	}
	return ""
}

func (m *model) cleanupDevice() {
	if m.dev != nil {
		m.dev.Close()
		m.dev = nil
	}
}

func isButtonTarget(t string) bool {
	_, ok := buttonMap[t]
	return ok
}
