package ui

import (
	"fmt"
	"runtime"
	"strconv"
	"time"

	"resource-lab/runner"
	"resource-lab/workload"
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
	choice bool // a two-value option: Enter and the arrows toggle it
	get    func(*runner.Config) int64
	set    func(*runner.Config, int64)
	format func(int64) string
}

// adjust moves the value by d steps. Crossing zero stops at zero, so "none"
// (-1) and "0" are both reachable in fields that have them.
func (f field) adjust(c *runner.Config, d int64) {
	old := f.get(c)
	if f.choice {
		f.set(c, 1-old)
		return
	}
	v := old + d*f.step
	if old != 0 && (old < 0) != (v < 0) {
		v = 0
	}
	f.set(c, clamp(v, f.min, f.max))
}

// setText sets the value typed by the user.
func (f field) setText(c *runner.Config, text string) error {
	v, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		return fmt.Errorf("%q is not a number", text)
	}
	if v < f.min || v > f.max {
		return fmt.Errorf("%s must be between %d and %d", f.label, f.min, f.max)
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
		Limits:       limitsNone(),
		Timeout:      time.Minute,
	}
}

func newFields() []field {
	cpus := int64(runtime.NumCPU())
	return []field{
		{
			label: "Threads", help: "workers that share the CPU work",
			min: 1, max: 1024, step: 1,
			get:    func(c *runner.Config) int64 { return int64(c.Threads) },
			set:    func(c *runner.Config, v int64) { c.Threads = int(v) },
			format: func(v int64) string { return strconv.FormatInt(v, 10) },
		},
		{
			label: "Worker type", help: "goroutine: tasks of the Go runtime (M:N)  os-thread: one OS thread each (1:1)",
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
			label: "Memory to use", help: "MiB the worker reserves with mmap and then touches page by page",
			min: 0, max: 1 << 16, step: 64,
			get:    func(c *runner.Config) int64 { return int64(c.MemoryMiB) },
			set:    func(c *runner.Config, v int64) { c.MemoryMiB = int(v) },
			format: noneOr("%d MiB"),
		},
		{
			label: "CPU work", help: "millions of iterations, split among the threads",
			min: 0, max: 1_000_000, step: 500,
			get:    func(c *runner.Config) int64 { return c.WorkMillions },
			set:    func(c *runner.Config, v int64) { c.WorkMillions = v },
			format: noneOr("%d M"),
		},
		{
			label: "CPU limit", help: "percent of ONE CPU the test may use (cpu.max); 200 = two CPUs; none = no limit",
			min: 0, max: 100 * cpus, step: 25,
			get:    func(c *runner.Config) int64 { return int64(c.Limits.CPUPercent) },
			set:    func(c *runner.Config, v int64) { c.Limits.CPUPercent = int(v) },
			format: noneOr("%d %%"),
		},
		{
			label: "Memory limit", help: "most RAM the test may use (memory.max), in MiB; none = no limit",
			min: 0, max: 1 << 16, step: 64,
			get:    func(c *runner.Config) int64 { return c.Limits.MemoryBytes >> 20 },
			set:    func(c *runner.Config, v int64) { c.Limits.MemoryBytes = v << 20 },
			format: noneOr("%d MiB"),
		},
		{
			label: "Swap limit", help: "most swap the test may use (memory.swap.max), in MiB; -1 = none, 0 = no swap",
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
					return "none"
				case v == 0:
					return "0 (no swap)"
				}
				return fmt.Sprintf("%d MiB", v)
			},
		},
		{
			label: "Time limit", help: "seconds before the test is stopped; none = never",
			min: 0, max: 3600, step: 10,
			get:    func(c *runner.Config) int64 { return int64(c.Timeout / time.Second) },
			set:    func(c *runner.Config, v int64) { c.Timeout = time.Duration(v) * time.Second },
			format: noneOr("%d s"),
		},
	}
}

// noneOr formats 0 as "none" and anything else with format.
func noneOr(format string) func(int64) string {
	return func(v int64) string {
		if v == 0 {
			return "none"
		}
		return fmt.Sprintf(format, v)
	}
}
