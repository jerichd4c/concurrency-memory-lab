package cgroup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// cpuPeriod is the length of a CPU quota period, in microseconds. The
// kernel lets the group run for quota microseconds in every period.
const cpuPeriod = 100000

// Limits are the limits requested for a test run.
type Limits struct {
	CPUPercent  int   // 0: no limit; 100: one full CPU; 250: two and a half CPUs
	MemoryBytes int64 // 0: no limit
	SwapBytes   int64 // < 0: no limit; 0: no swap at all; > 0: at most this much swap
}

// LimitResult tells whether one requested limit is really in force.
type LimitResult struct {
	Name      string // "cpu", "memory" or "swap"
	Requested string // the requested value, for people
	Applied   bool
	Err       error // why it was not applied
}

func (l LimitResult) String() string {
	if l.Applied {
		return fmt.Sprintf("%s %s: applied", l.Name, l.Requested)
	}
	return fmt.Sprintf("%s %s: NOT applied (%v)", l.Name, l.Requested, l.Err)
}

// setting is the cgroup file write that applies one requested limit.
type setting struct {
	name, requested, file, value string
}

// settings lists only the limits that were asked for.
func (l Limits) settings() []setting {
	var s []setting
	if l.CPUPercent > 0 {
		quota := int64(l.CPUPercent) * cpuPeriod / 100
		s = append(s, setting{"cpu", fmt.Sprintf("%d%% of one CPU", l.CPUPercent),
			"cpu.max", fmt.Sprintf("%d %d", quota, cpuPeriod)})
	}
	if l.MemoryBytes > 0 {
		s = append(s, setting{"memory", mib(l.MemoryBytes),
			"memory.max", fmt.Sprint(pageAlign(l.MemoryBytes))})
	}
	if l.SwapBytes >= 0 {
		s = append(s, setting{"swap", mib(l.SwapBytes),
			"memory.swap.max", fmt.Sprint(pageAlign(l.SwapBytes))})
	}
	return s
}

// Apply writes each requested limit and reads it back, so a limit is only
// reported as applied when the kernel really holds the value.
func (g *Group) Apply(l Limits) []LimitResult {
	var results []LimitResult
	for _, s := range l.settings() {
		r := LimitResult{Name: s.name, Requested: s.requested}
		path := filepath.Join(g.Dir, s.file)
		if err := os.WriteFile(path, []byte(s.value), 0o644); err != nil {
			r.Err = explain(err)
		} else if got, err := os.ReadFile(path); err != nil {
			r.Err = err
		} else if v := strings.TrimSpace(string(got)); v != s.value {
			r.Err = fmt.Errorf("%s holds %q instead of %q", s.file, v, s.value)
		} else {
			r.Applied = true
		}
		results = append(results, r)
	}
	return results
}

// NotApplied reports every requested limit as not applied because of err,
// for when the group itself could not be created.
func (l Limits) NotApplied(err error) []LimitResult {
	var results []LimitResult
	for _, s := range l.settings() {
		results = append(results, LimitResult{Name: s.name, Requested: s.requested, Err: err})
	}
	return results
}

// pageAlign rounds down to a whole number of pages, as the kernel does with
// memory limits; writing the rounded value lets Apply compare it exactly.
func pageAlign(b int64) int64 {
	page := int64(os.Getpagesize())
	return b / page * page
}

func mib(b int64) string {
	return fmt.Sprintf("%d MiB", b>>20)
}
