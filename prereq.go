package main

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

const vigemClientDLL = "ViGEmClient.dll"

// prereqStatus tracks prerequisite detection results.
type prereqStatus struct {
	ViGEmBusOK      bool
	ViGEmClientOK   bool
	HidHideOK       bool
	ViGEmClientPath string
}

// detectPrereqs checks for ViGEmBus, ViGEmClient.dll, and HidHide.
func detectPrereqs() prereqStatus {
	var s prereqStatus

	// ViGEmBus: check registry for the kernel service.
	k, err := registry.OpenKey(registry.LOCAL_MACHINE,
		`SYSTEM\CurrentControlSet\Services\ViGEmBus`, registry.QUERY_VALUE)
	if err == nil {
		k.Close()
		s.ViGEmBusOK = true
	}

	// HidHide: check registry for the kernel service (recommended, not required).
	k2, err := registry.OpenKey(registry.LOCAL_MACHINE,
		`SYSTEM\CurrentControlSet\Services\HidHide`, registry.QUERY_VALUE)
	if err == nil {
		k2.Close()
		s.HidHideOK = true
	}

	// ViGEmClient.dll: check next to executable, then PATH.
	exe, _ := os.Executable()
	nextToExe := filepath.Join(filepath.Dir(exe), vigemClientDLL)
	if _, err := os.Stat(nextToExe); err == nil {
		s.ViGEmClientOK = true
		s.ViGEmClientPath = nextToExe
	} else if sysPath := findInPath(vigemClientDLL); sysPath != "" {
		s.ViGEmClientOK = true
		s.ViGEmClientPath = sysPath
	} else {
		s.ViGEmClientPath = nextToExe
	}

	return s
}

func findInPath(filename string) string {
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if dir == "" {
			continue
		}
		candidate := filepath.Join(dir, filename)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}
