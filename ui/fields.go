package ui

import (
	"fmt"
	"runtime"
	"strconv"
	"time"

	"concurrency-memory-lab/runner"
	"concurrency-memory-lab/workload"
)

// field is one setting of the configuration form. Every setting is stored
// as an int64 so all of them are edited the same way: arrows change it by
// step, Enter lets the user type a number.
type field struct {
	label  string
	help   string // shown under the form while the field is selected
	min    int64
	max    int64
	step   int64
	choice bool // a list of options (min to max): Enter and the arrows go through them
	get    func(*runner.Config) int64
	set    func(*runner.Config, int64)
	format func(int64) string
}

// adjust moves the value by d steps. Crossing zero stops at zero, so "none"
// (-1) and "0" are both reachable in fields that have them. A choice goes
// round: after the last option comes the first one again.
func (f field) adjust(c *runner.Config, d int64) {
	old := f.get(c)
	if f.choice {
		n := f.max - f.min + 1
		f.set(c, f.min+((old-f.min+d)%n+n)%n)
		return
	}
	v := old + d*f.step
	if old != 0 && (old < 0) != (v < 0) {
		v = 0
	}
	f.set(c, clamp(v, f.min, f.max))
}

// setText sets the value typed by the user. t gives the error messages.
func (f field) setText(c *runner.Config, typed string, t *text) error {
	v, err := strconv.ParseInt(typed, 10, 64)
	if err != nil {
		return fmt.Errorf(t.notNumberF, typed)
	}
	if v < f.min || v > f.max {
		return fmt.Errorf(t.betweenF, f.label, f.min, f.max)
	}
	f.set(c, v)
	return nil
}

func clamp(v, lo, hi int64) int64 {
	return max(lo, min(v, hi))
}

// defaultConfig is the configuration shown when the program starts.
func defaultConfig() runner.Config {
	return runner.Config{
		Threads:      4,
		Mode:         workload.Goroutines,
		MemoryMiB:    256,
		WorkMillions: 2000,
		Sync:         workload.SyncOff,
		Limits:       limitsNone(),
		Timeout:      time.Minute,
	}
}

// newFields builds the form with the labels and help texts of t. The order
// is the order of t.labels and t.helps.
func newFields(t *text) []field {
	cpus := int64(runtime.NumCPU())
	fields := []field{
		{
			min: 1, max: runner.MaxThreads, step: 1,
			get:    func(c *runner.Config) int64 { return int64(c.Threads) },
			set:    func(c *runner.Config, v int64) { c.Threads = int(v) },
			format: func(v int64) string { return strconv.FormatInt(v, 10) },
		},
		{
			min: 0, max: 1, step: 1, choice: true,
			get: func(c *runner.Config) int64 {
				if c.Mode == workload.OSThreads {
					return 1
				}
				return 0
			},
			set: func(c *runner.Config, v int64) {
				c.Mode = workload.Goroutines
				if v == 1 {
					c.Mode = workload.OSThreads
				}
			},
			format: func(v int64) string {
				if v == 1 {
					return string(workload.OSThreads)
				}
				return string(workload.Goroutines)
			},
		},
		{
			min: 0, max: 1 << 16, step: 64,
			get:    func(c *runner.Config) int64 { return int64(c.MemoryMiB) },
			set:    func(c *runner.Config, v int64) { c.MemoryMiB = int(v) },
			format: noneOr(t.none, "%d MiB"),
		},
		{
			min: 0, max: 1_000_000, step: 500,
			get:    func(c *runner.Config) int64 { return c.WorkMillions },
			set:    func(c *runner.Config, v int64) { c.WorkMillions = v },
			format: noneOr(t.none, "%d M"),
		},
		{
			min: 0, max: int64(len(syncModes) - 1), step: 1, choice: true,
			get: func(c *runner.Config) int64 {
				for i, m := range syncModes {
					if m == c.Sync {
						return int64(i)
					}
				}
				return 0 // "" is off
			},
			set:    func(c *runner.Config, v int64) { c.Sync = syncModes[v] },
			format: func(v int64) string { return t.syncLabels[v] },
		},
		{
			min: 0, max: 100 * cpus, step: 25,
			get:    func(c *runner.Config) int64 { return int64(c.Limits.CPUPercent) },
			set:    func(c *runner.Config, v int64) { c.Limits.CPUPercent = int(v) },
			format: noneOr(t.none, "%d %%"),
		},
		{
			min: 0, max: 1 << 16, step: 64,
			get:    func(c *runner.Config) int64 { return c.Limits.MemoryBytes >> 20 },
			set:    func(c *runner.Config, v int64) { c.Limits.MemoryBytes = v << 20 },
			format: noneOr(t.none, "%d MiB"),
		},
		{
			min: -1, max: 1 << 16, step: 64,
			get: func(c *runner.Config) int64 {
				if c.Limits.SwapBytes < 0 {
					return -1
				}
				return c.Limits.SwapBytes >> 20
			},
			set: func(c *runner.Config, v int64) {
				c.Limits.SwapBytes = -1
				if v >= 0 {
					c.Limits.SwapBytes = v << 20
				}
			},
			format: func(v int64) string {
				switch {
				case v < 0:
					return t.none
				case v == 0:
					return t.noSwap
				}
				return fmt.Sprintf("%d MiB", v)
			},
		},
		{
			min: 0, max: 3600, step: 10,
			get:    func(c *runner.Config) int64 { return int64(c.Timeout / time.Second) },
			set:    func(c *runner.Config, v int64) { c.Timeout = time.Duration(v) * time.Second },
			format: noneOr(t.none, "%d s"),
		},
	}
	for i := range fields {
		fields[i].label, fields[i].help = t.labels[i], t.helps[i]
	}
	return fields
}

// The options of the "Shared counter" field, in the order of t.syncLabels.
var syncModes = []workload.SyncMode{workload.SyncOff, workload.SyncNone, workload.SyncMutex, workload.SyncAtomic}

// noneOr formats 0 as none and anything else with format.
func noneOr(none, format string) func(int64) string {
	return func(v int64) string {
		if v == 0 {
			return none
		}
		return fmt.Sprintf(format, v)
	}
}
