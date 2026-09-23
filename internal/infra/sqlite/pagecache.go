package sqlite

import "runtime"

// SQLite's page cache and mmap window are sized here, in one place, for every
// connection in both pools.
//
// These are cgo allocations and a file mapping: the Go allocator never sees
// them, so GOMEMLIMIT cannot restrain them, but RSS and the container's
// memory.current both charge them. Sizing them per-connection — as a literal
// repeated in the write pragma and the read DSN — meant the total scaled with
// the pool and nothing bounded it. On a single-vCPU host, which sizes the read
// pool at its floor of 2, that reserved 3 x 64MiB = 192MiB of a 2GB box before
// a single row was read.
const (
	// pageCacheBudgetKiB is the total page cache across ALL connections.
	pageCacheBudgetKiB = 48 * 1024

	// minCacheKiB is SQLite's own default (2MiB). A pool wide enough to divide
	// the budget below this gets the default rather than a starved cache; such a
	// host has the cores to justify the extra memory.
	minCacheKiB = 2 * 1024

	// mmapBytes maps part of the database file into the address space so page
	// reads skip a syscall and a copy. It is virtual address space rather than
	// committed RAM — but cgroup v2 charges the pages it faults in as file
	// memory, so on a small container it is not free and is sized accordingly.
	mmapBytes = 64 * 1024 * 1024
)

// perConnectionCacheKiB divides the page cache budget across the pool.
func perConnectionCacheKiB(connections int) int {
	if connections <= 1 {
		return pageCacheBudgetKiB
	}
	per := pageCacheBudgetKiB / connections
	if per < minCacheKiB {
		return minCacheKiB
	}
	return per
}

// readPoolSize is how many concurrent readers the pool allows. Reads run
// concurrently with the writer and with each other under WAL, so a long
// analysis query does not block ingestion.
//
// It follows GOMAXPROCS rather than NumCPU: under a container CPU quota
// NumCPU still reports every core on the HOST, so a 1-vCPU container would
// open a pool sized for the whole machine and reserve page cache for it.
func readPoolSize() int {
	if n := runtime.GOMAXPROCS(0); n > 2 {
		return n
	}
	return 2
}
