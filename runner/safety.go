package runner

import (
	"fmt"

	"concurrency-memory-lab/metrics"
)

// MaxThreads is the most workers a test may start.
const MaxThreads = 1024

// SafeMemory returns how much RAM and swap a test may take right now: half
// of the RAM that is available and half of the swap that is free, so the
// rest of the system always keeps room.
func SafeMemory(sys metrics.System) (ram, swap uint64) {
	return sys.MemAvailableBytes / 2, sys.SwapFreeBytes / 2
}

// checkMemory refuses a test that could take too much of the machine's
// memory. memLimit and swapLimit are the limits really in force: 0 and -1
// when none applies, because then nothing stops the worker.
//
// Without a memory limit everything the worker touches stays in RAM. With
// one, at most memLimit stays in RAM and the rest goes to swap, up to the
// swap limit; past that the kernel kills the worker inside its cgroup,
// which is safe for the system.
func checkMemory(memBytes, memLimit, swapLimit int64, sys metrics.System) error {
	ram, toSwap := memBytes, int64(0)
	if memLimit > 0 && memBytes > memLimit {
		ram, toSwap = memLimit, memBytes-memLimit
		if swapLimit >= 0 {
			toSwap = min(toSwap, swapLimit)
		}
		if sys.SwapTotalBytes == 0 {
			toSwap = 0 // no swap: the worker is killed when it reaches the limit
		}
	}
	safeRAM, safeSwap := SafeMemory(sys)
	if uint64(ram) > safeRAM {
		return &UnsafeError{MiB: uint64(ram) >> 20, MaxMiB: safeRAM >> 20}
	}
	if uint64(toSwap) > safeSwap {
		return &UnsafeError{Swap: true, MiB: uint64(toSwap) >> 20, MaxMiB: safeSwap >> 20}
	}
	return nil
}

// UnsafeError is why a test was refused: it would put more memory in RAM,
// or in swap, than is safe. Its fields let the interface explain it in its
// own language.
type UnsafeError struct {
	Swap   bool   // the problem is swap, not RAM
	MiB    uint64 // what the test would use
	MaxMiB uint64 // the safe maximum
}

func (e *UnsafeError) Error() string {
	if e.Swap {
		return fmt.Sprintf("unsafe: %d MiB to swap, safe max %d MiB (half of free swap)", e.MiB, e.MaxMiB)
	}
	return fmt.Sprintf("unsafe: %d MiB in RAM, safe max %d MiB (half of available RAM)", e.MiB, e.MaxMiB)
}
