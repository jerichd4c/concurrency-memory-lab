package runner

import (
	"errors"
	"slices"
	"syscall"
	"testing"
	"time"

	"concurrency-memory-lab/cgroup"
	"concurrency-memory-lab/workload"
)

var errTest = errors.New("test error")

func validConfig() Config {
	return Config{
		Threads:      4,
		Mode:         workload.OSThreads,
		MemoryMiB:    64,
		WorkMillions: 500,
		Limits:       cgroup.Limits{SwapBytes: -1},
	}
}

func TestWorkerArgs(t *testing.T) {
	want := []string{"-threads", "4", "-mode", "os-thread", "-mem", "64", "-work", "500"}
	if got := validConfig().WorkerArgs(); !slices.Equal(got, want) {
		t.Errorf("WorkerArgs = %v, want %v", got, want)
	}
	c := validConfig()
	c.Sync = workload.SyncMutex
	want = append(want, "-sync", "mutex")
	if got := c.WorkerArgs(); !slices.Equal(got, want) {
		t.Errorf("WorkerArgs with sync = %v, want %v", got, want)
	}
}

func TestParseWork(t *testing.T) {
	out := "pid=1 mode=goroutine threads=4\n" +
		"cpu work: 2000000000 iterations in 1.272s (checksum 0e8291f3e00be6c0)\nresult: ok\n"
	w := parseWork(out)
	if !w.Reported || w.Shared || w.Done != 2_000_000_000 || w.Elapsed != 1272*time.Millisecond {
		t.Errorf("cpu work: %+v", w)
	}

	out = "shared counter (none): 1234567 of 4000000 increments, lost 2765433 (69.1%) in 120ms\n"
	w = parseWork(out)
	if !w.Reported || !w.Shared || w.Counter != 1234567 || w.Done != 4_000_000 ||
		w.Elapsed != 120*time.Millisecond || w.Lost() != 2765433 {
		t.Errorf("shared counter: %+v, lost %d", w, w.Lost())
	}

	for _, bad := range []string{"", "result: stopped\n", "cpu work: many iterations in 1s\n"} {
		if w := parseWork(bad); w.Reported {
			t.Errorf("parseWork(%q) = %+v, want not reported", bad, w)
		}
	}
}

func TestValidate(t *testing.T) {
	if err := validConfig().Validate(); err != nil {
		t.Fatalf("valid config: %v", err)
	}
	bad := []func(*Config){
		func(c *Config) { c.Threads = 0 },
		func(c *Config) { c.Threads = MaxThreads + 1 },
		func(c *Config) { c.Mode = "fiber" },
		func(c *Config) { c.Sync = "semaphore" },
		func(c *Config) { c.MemoryMiB = -1 },
		func(c *Config) { c.WorkMillions = -1 },
		func(c *Config) { c.Limits.CPUPercent = -5 },
		func(c *Config) { c.Limits.MemoryBytes = -1 },
		func(c *Config) { c.Timeout = -time.Second },
	}
	for i, change := range bad {
		c := validConfig()
		change(&c)
		if err := c.Validate(); err == nil {
			t.Errorf("case %d: Validate accepted %+v", i, c)
		}
	}
}

func TestClassify(t *testing.T) {
	tests := []struct {
		exit   int
		signal syscall.Signal
		reason stopReason
		ooms   uint64
		want   Outcome
	}{
		{0, 0, reasonNone, 0, OutcomeCompleted},
		{0, 0, reasonUser, 0, OutcomeCompleted}, // finished before the stop arrived
		{-1, syscall.SIGKILL, reasonNone, 1, OutcomeOOM},
		{-1, syscall.SIGKILL, reasonUser, 1, OutcomeOOM},
		{workload.ExitStopped, 0, reasonTimeout, 0, OutcomeTimeout},
		{-1, syscall.SIGKILL, reasonTimeout, 0, OutcomeTimeout},
		{workload.ExitStopped, 0, reasonUser, 0, OutcomeStopped},
		{workload.ExitStopped, 0, reasonNone, 0, OutcomeStopped}, // SIGTERM sent by someone else
		{-1, syscall.SIGKILL, reasonNone, 0, OutcomeKilled},
		{workload.ExitError, 0, reasonNone, 0, OutcomeFailed},
	}
	for _, tt := range tests {
		if got := classify(tt.exit, tt.signal, tt.reason, tt.ooms); got != tt.want {
			t.Errorf("classify(%d, %v, %d, %d) = %q, want %q", tt.exit, tt.signal, tt.reason, tt.ooms, got, tt.want)
		}
	}
}

func TestStartRejectsInvalidConfig(t *testing.T) {
	c := validConfig()
	c.Threads = 0
	if _, err := Start("/bin/true", c); err == nil {
		t.Error("Start accepted an invalid config")
	}
}
