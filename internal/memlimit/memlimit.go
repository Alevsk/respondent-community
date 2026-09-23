// Package memlimit derives the Go runtime's soft memory limit from the memory
// limit the container imposes on this process.
//
// Without it the Go GC sizes the heap against the HOST's RAM: on a shared box
// it will happily grow past the container's share, and the kernel — not the
// runtime — is the first thing in the chain that says no. A global OOM sweep
// then picks a victim anywhere on the machine, which is why an unbounded
// container takes the whole host down with it instead of merely restarting.
package memlimit

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ErrNoLimit reports that the process runs without a container memory limit.
// It is not a failure: it means nothing has stated how much memory this
// process may use, and the caller should say so rather than invent a number.
// A fraction of host RAM would be the wrong guess on a shared host.
var ErrNoLimit = errors.New("no cgroup memory limit is set for this process")

// headroomRatio is the share of the container limit handed to the Go runtime.
// The remainder covers what GOMEMLIMIT does not account for: the binary's text
// and rodata, thread stacks, and SQLite's page cache, which is cgo-malloc'd
// and therefore invisible to the Go allocator.
const headroomRatio = 0.8

// v1Unlimited is how cgroup v1 spells "no limit": int64 max rounded down to a
// page boundary. Anything at or above it is not a real limit.
const v1Unlimited int64 = 0x7FFFFFFFFFFFF000

// Detect returns the container memory limit in bytes, reading cgroup v2 first
// and falling back to v1. root is the filesystem root, "/" in production.
func Detect(root string) (int64, error) {
	if limit, ok := readLimit(filepath.Join(root, "sys/fs/cgroup/memory.max")); ok {
		return limit, nil
	}
	if limit, ok := readLimit(filepath.Join(root, "sys/fs/cgroup/memory/memory.limit_in_bytes")); ok {
		return limit, nil
	}
	return 0, ErrNoLimit
}

// readLimit parses one cgroup limit file. It reports ok=false for a missing
// file, an unparseable value, or either spelling of "unlimited".
func readLimit(path string) (int64, bool) {
	data, err := os.ReadFile(path) //nolint:gosec // path is derived from a fixed cgroup layout
	if err != nil {
		return 0, false
	}
	text := strings.TrimSpace(string(data))
	if text == "max" {
		return 0, false
	}
	limit, err := strconv.ParseInt(text, 10, 64)
	if err != nil || limit <= 0 || limit >= v1Unlimited {
		return 0, false
	}
	return limit, true
}

// Budget converts a container memory limit into the Go runtime's soft limit.
func Budget(containerLimit int64) int64 {
	return int64(float64(containerLimit) * headroomRatio)
}

// Describe renders a byte count for logs.
func Describe(bytes int64) string {
	return fmt.Sprintf("%.0fMiB", float64(bytes)/(1024*1024))
}
