package verify

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRunExecutesInOrderFromRoot(t *testing.T) {
	root := t.TempDir()
	commands := []string{"echo first>>order.txt", "echo second>>order.txt"}
	result := Run(root, commands, &bytes.Buffer{}, &bytes.Buffer{})
	if !result.Success() || result.Passed != 2 || result.Total != 2 {
		t.Fatalf("unexpected result: %+v", result)
	}
	contents, err := os.ReadFile(filepath.Join(root, "order.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(contents)) != "first\nsecond" && strings.TrimSpace(string(contents)) != "first\r\nsecond" {
		t.Fatalf("commands did not run in order: %q", contents)
	}
}

func TestRunStopsAtFirstFailure(t *testing.T) {
	root := t.TempDir()
	failure := "false"
	if runtime.GOOS == "windows" {
		failure = "exit /b 7"
	}
	result := Run(root, []string{failure, "echo should-not-run>>marker.txt"}, &bytes.Buffer{}, &bytes.Buffer{})
	if result.Success() || result.Passed != 0 || result.FailedPosition != 1 || result.FailedCommand != failure {
		t.Fatalf("unexpected failure result: %+v", result)
	}
	if _, err := os.Stat(filepath.Join(root, "marker.txt")); !os.IsNotExist(err) {
		t.Fatalf("later command ran, stat error: %v", err)
	}
}
