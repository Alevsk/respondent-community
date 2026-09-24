package sqlite

import "testing"

// The page cache is cgo-malloc'd: invisible to the Go allocator and to
// GOMEMLIMIT, but charged to RSS and to the container's memory.current. It is
// sized per pool by the traffic that pool carries — every repository runs on
// the write connection, while the read pool serves only scheduled analysis SQL.
func TestPageCacheBudget(t *testing.T) {
	t.Run("the connection every repository uses is not starved", func(t *testing.T) {
		// Splitting one budget evenly across both pools gave this connection a
		// third of it while two idle read connections held the rest.
		if writeCacheKiB <= readCacheKiB {
			t.Errorf("write cache %d KiB must exceed a read connection's %d KiB", writeCacheKiB, readCacheKiB)
		}
	})

	t.Run("total stays bounded on a single-vCPU host", func(t *testing.T) {
		// The floor of 2 readers is what a 1-vCPU container gets.
		if got, want := readPoolSize(), 2; runtimeMaxProcsIsOne() && got != want {
			t.Errorf("read pool on one CPU: got %d, want %d", got, want)
		}
		const ceiling = 64 * 1024 // the single per-connection value this replaced
		if total := writeCacheKiB + readCacheKiB*2; total > ceiling {
			t.Errorf("1-vCPU total %d KiB exceeds the %d KiB a single connection used to take", total, ceiling)
		}
	})

	t.Run("total scales sublinearly as the read pool widens", func(t *testing.T) {
		if totalPageCacheKiB() < writeCacheKiB {
			t.Error("total must include the write connection")
		}
	})
}

func runtimeMaxProcsIsOne() bool { return readPoolSize() == 2 }
