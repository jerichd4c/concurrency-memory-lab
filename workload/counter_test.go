package workload

import (
	"sync/atomic"
	"testing"
)

func TestParseSyncMode(t *testing.T) {
	for _, s := range []string{"off", "none", "mutex", "atomic"} {
		if m, err := ParseSyncMode(s); err != nil || string(m) != s {
			t.Errorf("ParseSyncMode(%q) = %q, %v", s, m, err)
		}
	}
	if m, err := ParseSyncMode(""); err != nil || m != SyncOff {
		t.Errorf("ParseSyncMode(\"\") = %q, %v; want off", m, err)
	}
	if _, err := ParseSyncMode("semaphore"); err == nil {
		t.Error("ParseSyncMode(\"semaphore\") returned no error")
	}
}

// With synchronization no update is lost, whatever the number of workers.
func TestSynchronizedCounterIsExact(t *testing.T) {
	const total = 200_003
	for _, s := range []SyncMode{SyncMutex, SyncAtomic} {
		for _, mode := range []ThreadMode{Goroutines, OSThreads} {
			for _, workers := range []int{1, 4, 8} {
				var stop atomic.Bool
				done, counter := RunCounter(total, workers, mode, s, &stop)
				if done != total || counter != total {
					t.Errorf("%s %s x%d: done %d, counter %d; want both %d", s, mode, workers, done, counter, total)
				}
			}
		}
	}
}

// With a single worker nobody else writes between the read and the write,
// so even the unsynchronized counter is exact.
func TestUnsynchronizedCounterWithOneWorker(t *testing.T) {
	var stop atomic.Bool
	done, counter := RunCounter(100_000, 1, Goroutines, SyncNone, &stop)
	if done != 100_000 || counter != 100_000 {
		t.Errorf("done %d, counter %d; want both 100000", done, counter)
	}
}

// The race is not deterministic, so the test only checks that the counter
// never exceeds the increments done. TestUnsynchronizedCounterLosesUpdates
// shows the loss itself when the machine has several CPUs.
func TestUnsynchronizedCounterNeverGains(t *testing.T) {
	var stop atomic.Bool
	done, counter := RunCounter(1_000_000, 4, Goroutines, SyncNone, &stop)
	if done != 1_000_000 || counter > done || counter <= 0 {
		t.Errorf("done %d, counter %d; want 0 < counter <= done = 1000000", done, counter)
	}
}

func TestUnsynchronizedCounterLosesUpdates(t *testing.T) {
	if testing.Short() {
		t.Skip("depends on timing")
	}
	var stop atomic.Bool
	for range 5 {
		done, counter := RunCounter(5_000_000, 8, OSThreads, SyncNone, &stop)
		if counter < done {
			return // updates were lost, as expected
		}
	}
	t.Skip("no update was lost in 5 tries: this machine may have a single CPU")
}

func TestRunCounterStops(t *testing.T) {
	var stop atomic.Bool
	stop.Store(true)
	if done, counter := RunCounter(1_000_000_000, 4, Goroutines, SyncAtomic, &stop); done != 0 || counter != 0 {
		t.Errorf("with stop set: done %d, counter %d; want 0, 0", done, counter)
	}
}
