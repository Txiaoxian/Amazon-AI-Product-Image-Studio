package sse

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSSEPackageDoesNotIntroducePollingMarkers(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test file path")
	}
	root := filepath.Dir(file)
	forbidden := []string{"setInterval", "repeated fetch"}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := string(body)
		for _, marker := range forbidden {
			if strings.Contains(text, marker) && filepath.Base(path) != "no_polling_test.go" {
				t.Fatalf("%s contains polling marker %q", path, marker)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("scan SSE package: %v", err)
	}
}
