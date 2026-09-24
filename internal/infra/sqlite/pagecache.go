package sqlite

import "runtime"

// SQLite's page cache and mmap window are sized here, in one place, for every
// connection in both pools.
//
// These are cgo allocations and a file mapping: the Go allocator never sees
// them, so GOMEMLIMIT cannot restrain them, but RSS and the container's
// memory.current both charge them. The value used to be a single literal
// repeated in the write pragma and the read DSN, so the total scaled with the
// pool and nothing bounded it — on a 1-vCPU host that reserved 3 x 64MiB.
//
// The two pools are sized by the traffic they carry, not by an even split.
// Every repository is opened on the write handle (see cmd/community/serve.go),
// so that one connection serves every snapshot, viewport, REST read and feeder
// write. The read pool is reached only by scheduled analysis SQL. Dividing one
// budget evenly across both would starve the hot connection to fund idle ones.
const (
	// writeCacheKiB is the page cache for the connection every repository uses.
	writeCacheKiB = 48 * 1024

	// readCacheKiB is the page cache for each read-pool connection. These serve
	// occasional analysis queries, so they get SQLite's own default rather than
	// a share of the budget.
	readCacheKiB = 2 * 1024

	// mmapBytes maps part of the database file into the address space so page
	// reads skip a syscall and a copy. It is virtual address space rather than
	// committed RAM — but cgroup v2 charges the pages it faults in as file
	// memory, so on a small container it is not free and is sized accordingly.
	mmapBytes = 64 * 1024 * 1024
)

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

// totalPageCacheKiB is the ceiling this configuration can reserve across both
// pools. Exposed for the test that pins the budget.
func totalPageCacheKiB() int {
	return writeCacheKiB + readCacheKiB*readPoolSize()
}
