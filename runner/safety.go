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
		return fmt.Errorf("unsafe: %d MiB in RAM, safe max %d MiB (half of available RAM)", ram>>20, safeRAM>>20)
	}
	if uint64(toSwap) > safeSwap {
		return fmt.Errorf("unsafe: %d MiB to swap, safe max %d MiB (half of free swap)", toSwap>>20, safeSwap>>20)
	}
	return nil
}
