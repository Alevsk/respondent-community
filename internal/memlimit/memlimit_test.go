package memlimit

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDetect(t *testing.T) {
	t.Run("cgroup v2 limit", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, root, "sys/fs/cgroup/memory.max", "943718400\n")
		got, err := Detect(root)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != 943718400 {
			t.Errorf("got %d, want 943718400", got)
		}
	})

	t.Run("cgroup v2 unlimited reads as no limit", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, root, "sys/fs/cgroup/memory.max", "max\n")
		if _, err := Detect(root); !errors.Is(err, ErrNoLimit) {
			t.Errorf("expected ErrNoLimit, got %v", err)
		}
	})

	t.Run("falls back to cgroup v1", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, root, "sys/fs/cgroup/memory/memory.limit_in_bytes", "536870912\n")
		got, err := Detect(root)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != 536870912 {
			t.Errorf("got %d, want 536870912", got)
		}
	})

	t.Run("cgroup v1 sentinel reads as no limit", func(t *testing.T) {
		// v1 reports "unlimited" as a value near int64 max rounded to page size.
		root := t.TempDir()
		writeFile(t, root, "sys/fs/cgroup/memory/memory.limit_in_bytes", "9223372036854771712\n")
		if _, err := Detect(root); !errors.Is(err, ErrNoLimit) {
			t.Errorf("expected ErrNoLimit, got %v", err)
		}
	})

	t.Run("no cgroup files at all", func(t *testing.T) {
		if _, err := Detect(t.TempDir()); !errors.Is(err, ErrNoLimit) {
			t.Errorf("expected ErrNoLimit, got %v", err)
		}
	})
}

func TestBudget(t *testing.T) {
	// The Go soft limit must leave room for what the Go runtime does not
	// account: the binary, and SQLite's page cache, which cgo mallocs.
	got := Budget(943718400)
	if got >= 943718400 {
		t.Errorf("budget %d must be below the cgroup limit", got)
	}
	if got != int64(float64(943718400)*headroomRatio) {
		t.Errorf("got %d, want %d", got, int64(float64(943718400)*headroomRatio))
	}
}
