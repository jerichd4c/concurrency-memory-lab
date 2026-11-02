package workload

import (
	"slices"
	"sync/atomic"
	"testing"
)

func TestParseThreadMode(t *testing.T) {
	for _, s := range []string{"goroutine", "os-thread"} {
		if m, err := ParseThreadMode(s); err != nil || string(m) != s {
			t.Errorf("ParseThreadMode(%q) = %q, %v", s, m, err)
		}
	}
	if _, err := ParseThreadMode("fiber"); err == nil {
		t.Error("ParseThreadMode(\"fiber\") returned no error")
	}
}

func TestSplitWork(t *testing.T) {
	tests := []struct {
		total int64
		n     int
		want  []int64
	}{
		{10, 1, []int64{10}},
		{10, 3, []int64{4, 3, 3}},
		{12, 4, []int64{3, 3, 3, 3}},
		{2, 4, []int64{1, 1, 0, 0}},
	}
	for _, tt := range tests {
		if got := SplitWork(tt.total, tt.n); !slices.Equal(got, tt.want) {
			t.Errorf("SplitWork(%d, %d) = %v, want %v", tt.total, tt.n, got, tt.want)
		}
	}
}

func TestRunCPUDoesAllWork(t *testing.T) {
	const total = 1_000_003
	for _, mode := range []ThreadMode{Goroutines, OSThreads} {
		for _, workers := range []int{1, 3, 8} {
			var stop atomic.Bool
			if done, _ := RunCPU(total, workers, mode, &stop); done != total {
				t.Errorf("%s x%d: done %d, want %d", mode, workers, done, total)
			}
		}
	}
}

func TestRunCPUStops(t *testing.T) {
	var stop atomic.Bool
	stop.Store(true)
	if done, _ := RunCPU(1_000_000_000, 4, Goroutines, &stop); done != 0 {
		t.Errorf("RunCPU with stop set did %d iterations, want 0", done)
	}
}
