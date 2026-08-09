package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

const vigemClientDLL = "ViGEmClient.dll"

// prereqStatus tracks prerequisite detection results.
type prereqStatus struct {
	ViGEmBusOK      bool
	ViGEmClientOK   bool
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

// downloadViGEmClient fetches ViGEmClient.dll from our repo and places it next to the exe.
func downloadViGEmClient(targetDir string) (string, error) {
	target := filepath.Join(targetDir, vigemClientDLL)
	url := "https://raw.githubusercontent.com/ramilamparo/hid2xbox/master/ViGEmClient.dll"

	fmt.Printf("Downloading %s...\n", vigemClientDLL)
	if err := downloadFile(url, target); err != nil {
		return "", fmt.Errorf("download failed: %w", err)
	}
	fmt.Printf("  saved to %s\n", target)
	return target, nil
}

func downloadFile(url, target string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	f, err := os.Create(target)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	return err
}
