package workload

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
)

// SyncMode selects the CPU work of the test: independent work for every
// worker, or one counter shared by all of them, updated with or without
// synchronization.
type SyncMode string

const (
	SyncOff    SyncMode = "off"    // no shared data: the xorshift work of RunCPU
	SyncNone   SyncMode = "none"   // read, then write: updates get lost
	SyncMutex  SyncMode = "mutex"  // the increment runs inside a lock
	SyncAtomic SyncMode = "atomic" // the CPU does read+add+write as one step
)

// ParseSyncMode converts a flag value into a SyncMode. "" means off.
func ParseSyncMode(s string) (SyncMode, error) {
	switch SyncMode(s) {
	case "":
		return SyncOff, nil
	case SyncOff, SyncNone, SyncMutex, SyncAtomic:
		return SyncMode(s), nil
	}
	return "", fmt.Errorf("unknown sync mode %q (use off, none, mutex or atomic)", s)
}

// RunCounter makes the workers add 1 to one shared counter, total times in
// all, split among them. It returns the increments actually done (less than
// total if stop was set) and the final value of the counter: both are equal
// only if no update was lost.
func RunCounter(total int64, workers int, mode ThreadMode, syncMode SyncMode, stop *atomic.Bool) (done, counter int64) {
	var mu sync.Mutex
	var increment func()
	switch syncMode {
	case SyncNone:
		increment = func() {
			// Two separate steps: if another worker writes between them,
			// this write overwrites it and that update is lost. Each step
			// is atomic only so the compiler keeps one read and one write
			// per increment instead of merging the whole loop.
			v := atomic.LoadInt64(&counter)
			atomic.StoreInt64(&counter, v+1)
		}
	case SyncMutex:
		increment = func() {
			mu.Lock()
			counter++
			mu.Unlock()
		}
	case SyncAtomic:
		increment = func() { atomic.AddInt64(&counter, 1) }
	default:
		panic("RunCounter needs a sync mode, got " + string(syncMode))
	}

	parts := SplitWork(total, workers)
	dones := make([]int64, workers)
	var wg sync.WaitGroup
	for w := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if mode == OSThreads {
				runtime.LockOSThread()
			}
			var i int64
			for ; i < parts[w]; i++ {
				if i&0xFFFF == 0 && stop.Load() {
					break
				}
				increment()
			}
			dones[w] = i
		}()
	}
	wg.Wait()

	for _, d := range dones {
		done += d
	}
	return done, counter
}
