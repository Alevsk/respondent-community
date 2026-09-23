package sqlite

import "testing"

// The page cache is cgo-malloc'd: it is invisible to the Go allocator and to
// GOMEMLIMIT, but the kernel and the cgroup both charge it. Sizing it
// per-connection meant the total scaled with the pool, so the smallest hosts —
// which size the pool at its floor of 2 readers — still reserved 192MB.
func TestPerConnectionCacheKiB(t *testing.T) {
	t.Run("splits one budget across the pool", func(t *testing.T) {
		// 1 writer + 2 readers on a single-vCPU droplet.
		if got, want := perConnectionCacheKiB(3), pageCacheBudgetKiB/3; got != want {
			t.Errorf("got %d KiB, want %d KiB", got, want)
		}
	})

	t.Run("total stays within the budget as the pool widens", func(t *testing.T) {
		for _, conns := range []int{1, 2, 3, 5, 9, 17} {
			total := perConnectionCacheKiB(conns) * conns
			if total > pageCacheBudgetKiB && perConnectionCacheKiB(conns) != minCacheKiB {
				t.Errorf("%d connections: total %d KiB exceeds budget %d KiB", conns, total, pageCacheBudgetKiB)
			}
		}
	})

	t.Run("never drops below SQLite's own default", func(t *testing.T) {
		if got := perConnectionCacheKiB(512); got != minCacheKiB {
			t.Errorf("got %d KiB, want the %d KiB floor", got, minCacheKiB)
		}
	})

	t.Run("rejects a nonsensical pool size", func(t *testing.T) {
		if got := perConnectionCacheKiB(0); got != pageCacheBudgetKiB {
			t.Errorf("got %d KiB, want the whole budget %d KiB", got, pageCacheBudgetKiB)
		}
	})
}
