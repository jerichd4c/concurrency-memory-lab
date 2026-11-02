package workload

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
)

// ThreadMode selects what kind of worker runs the CPU work.
type ThreadMode string

const (
	// Goroutines are tasks managed by the Go runtime: many goroutines share
	// a pool of OS threads, at most GOMAXPROCS running at once (M:N model).
	Goroutines ThreadMode = "goroutine"
	// OSThreads locks every worker to its own OS thread for its whole life
	// (1:1 model), so each worker is a real thread in /proc/<pid>/task.
	OSThreads ThreadMode = "os-thread"
)

// ParseThreadMode converts a flag value into a ThreadMode.
func ParseThreadMode(s string) (ThreadMode, error) {
	switch ThreadMode(s) {
	case Goroutines, OSThreads:
		return ThreadMode(s), nil
	}
	return "", fmt.Errorf("unknown thread mode %q (use %q or %q)", s, Goroutines, OSThreads)
}

// SplitWork divides total units of work among n workers. The first workers
// get one extra unit each until the remainder is used, so the sum is total.
func SplitWork(total int64, n int) []int64 {
	parts := make([]int64, n)
	base, extra := total/int64(n), total%int64(n)
	for i := range parts {
		parts[i] = base
		if int64(i) < extra {
			parts[i]++
		}
	}
	return parts
}

// RunCPU runs total iterations of a CPU-bound loop split across workers.
// It returns the iterations actually done (less than total if stop was set)
// and a checksum that keeps the compiler from removing the loop.
func RunCPU(total int64, workers int, mode ThreadMode, stop *atomic.Bool) (done int64, checksum uint64) {
	parts := SplitWork(total, workers)
	dones := make([]int64, workers)
	sums := make([]uint64, workers)

	var wg sync.WaitGroup
	for w := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if mode == OSThreads {
				// Never unlocked: when the goroutine ends, the runtime
				// terminates its thread instead of reusing it.
				runtime.LockOSThread()
			}
			seed := uint64(w+1) * 0x9E3779B97F4A7C15
			dones[w], sums[w] = spin(parts[w], seed, stop)
		}()
	}
	wg.Wait()

	for w := range workers {
		done += dones[w]
		checksum ^= sums[w]
	}
	return done, checksum
}

// spin runs n steps of a xorshift random generator: pure CPU work with no
// memory traffic. It checks stop every 65536 steps.
func spin(n int64, x uint64, stop *atomic.Bool) (int64, uint64) {
	var i int64
	for ; i < n; i++ {
		if i&0xFFFF == 0 && stop.Load() {
			break
		}
		x ^= x << 13
		x ^= x >> 7
		x ^= x << 17
	}
	return i, x
}
