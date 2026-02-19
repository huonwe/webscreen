package utils

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// GetADBPath returns the path to the ADB executable.
// It checks the current directory, then the system PATH.
// If not found, it downloads ADB from Google's repository.
func GetADBPath() (string, error) {
	exeName := "adb"
	if runtime.GOOS == "windows" {
		exeName = "adb.exe"
	}

	// 1. Check local directory
	localPath, err := filepath.Abs(exeName)
	if err == nil {
		if _, err := os.Stat(localPath); err == nil {
			return localPath, nil
		}
	}
	// 2. Check PATH
	if path, err := exec.LookPath("adb"); err == nil {
		return path, nil
	}

	// 3. Download
	fmt.Println("ADB not found. Downloading...")
	if err := downloadADB(); err != nil {
		return "", fmt.Errorf("failed to download ADB: %v", err)
	}

	// Return local path after download
	return localPath, nil
}

func downloadADB() error {
	var url string
	switch runtime.GOOS {
	case "windows":
		url = "https://dl.google.com/android/repository/platform-tools-latest-windows.zip"
	case "linux":
		url = "https://dl.google.com/android/repository/platform-tools-latest-linux.zip"
	case "darwin":
		url = "https://dl.google.com/android/repository/platform-tools-latest-darwin.zip"
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Create a temporary file for the zip
	tmpFile, err := os.CreateTemp("", "platform-tools-*.zip")
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name())

	_, err = io.Copy(tmpFile, resp.Body)
	if err != nil {
		return err
	}
	tmpFile.Close()

	// Unzip
	return unzipADB(tmpFile.Name())
}

func unzipADB(src string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		// We only need adb and its dependencies (dlls on windows)
		// They are inside "platform-tools/" folder in the zip
		name := f.Name
		if !strings.HasPrefix(name, "platform-tools/") {
			continue
		}

		baseName := filepath.Base(name)
		if baseName == "" {
			continue
		}

		// Filter files we need
		needed := false
		if baseName == "adb" || baseName == "adb.exe" {
			needed = true
		} else if runtime.GOOS == "windows" && (strings.HasSuffix(baseName, ".dll")) {
			// AdbWinApi.dll, AdbWinUsbApi.dll
			needed = true
		}

		if needed {
			// Extract to current directory
			outFile, err := os.OpenFile(baseName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return err
			}

			rc, err := f.Open()
			if err != nil {
				outFile.Close()
				return err
			}

			_, err = io.Copy(outFile, rc)
			outFile.Close()
			rc.Close()

			if err != nil {
				return err
			}
		}
	}
	return nil
}
