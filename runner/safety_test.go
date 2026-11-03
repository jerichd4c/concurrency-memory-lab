package runner

import (
	"strings"
	"testing"

	"concurrency-memory-lab/metrics"
)

const mib = 1 << 20

// machine has 4000 MiB of RAM available and 2000 MiB of free swap, so a
// test may take up to 2000 MiB of RAM and 1000 MiB of swap.
var machine = metrics.System{
	MemTotalBytes:     8000 * mib,
	MemAvailableBytes: 4000 * mib,
	SwapTotalBytes:    2048 * mib,
	SwapFreeBytes:     2000 * mib,
}

func TestSafeMemory(t *testing.T) {
	ram, swap := SafeMemory(machine)
	if ram != 2000*mib || swap != 1000*mib {
		t.Errorf("SafeMemory = %d, %d MiB; want 2000, 1000", ram/mib, swap/mib)
	}
}

func TestCheckMemory(t *testing.T) {
	tests := []struct {
		name                     string
		mem, memLimit, swapLimit int64
		sys                      metrics.System
		wantErr                  string // "" = allowed
	}{
		{"fits in RAM, no limit", 1500 * mib, 0, -1, machine, ""},
		{"too much RAM, no limit", 3000 * mib, 0, -1, machine, "3000 MiB in RAM, safe max 2000 MiB"},
		{"exactly the safe max", 2000 * mib, 0, -1, machine, ""},
		{"limit keeps RAM low, rest to swap", 1500 * mib, 1000 * mib, -1, machine, ""},
		{"limit too high itself", 3000 * mib, 2500 * mib, 0, machine, "2500 MiB in RAM"},
		{"too much would go to swap", 2500 * mib, 1000 * mib, -1, machine, "1500 MiB to swap, safe max 1000 MiB"},
		{"swap limit caps the swap", 2500 * mib, 1000 * mib, 500 * mib, machine, ""},
		{"no swap allowed: OOM is safe", 9000 * mib, 1000 * mib, 0, machine, ""},
		{"machine without swap: OOM is safe", 9000 * mib, 1000 * mib, -1,
			metrics.System{MemTotalBytes: 8000 * mib, MemAvailableBytes: 4000 * mib}, ""},
		{"limit larger than the memory used", 500 * mib, 1000 * mib, -1, machine, ""},
	}
	for _, tt := range tests {
		err := checkMemory(tt.mem, tt.memLimit, tt.swapLimit, tt.sys)
		switch {
		case tt.wantErr == "" && err != nil:
			t.Errorf("%s: refused: %v", tt.name, err)
		case tt.wantErr != "" && err == nil:
			t.Errorf("%s: allowed, want an error with %q", tt.name, tt.wantErr)
		case tt.wantErr != "" && !strings.Contains(err.Error(), tt.wantErr):
			t.Errorf("%s: error %q, want it to contain %q", tt.name, err, tt.wantErr)
		}
	}
}

func TestLimitsInForceIgnoresLimitsNotApplied(t *testing.T) {
	r := &Run{cfg: validConfig()}
	r.cfg.Limits.MemoryBytes = 128 * mib
	r.cfg.Limits.SwapBytes = 0
	r.limits = r.cfg.Limits.NotApplied(errTest)
	if mem, swap := r.limitsInForce(); mem != 0 || swap != -1 {
		t.Errorf("limits not applied: in force %d, %d; want 0, -1", mem, swap)
	}
	for i := range r.limits {
		r.limits[i].Applied = true
	}
	if mem, swap := r.limitsInForce(); mem != 128*mib || swap != 0 {
		t.Errorf("limits applied: in force %d, %d; want %d, 0", mem, swap, 128*mib)
	}
}
