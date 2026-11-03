// Package runner starts one test run: it creates the cgroup, applies the
// limits, launches the worker inside it, samples its measurements, stops it
// on request or timeout and collects the result.
package runner

import (
	"errors"
	"strconv"
	"time"

	"resource-lab/cgroup"
	"resource-lab/workload"
)

// Config describes one test run.
type Config struct {
	Threads      int
	Mode         workload.ThreadMode
	MemoryMiB    int               // memory the worker reserves and touches
	WorkMillions int64             // CPU work, in millions of iterations or increments
	Sync         workload.SyncMode // shared counter mode; "" is off
	Limits       cgroup.Limits
	Timeout      time.Duration // 0: no timeout
}

// Validate checks the values before anything is started.
func (c Config) Validate() error {
	if c.Threads < 1 {
		return errors.New("threads must be at least 1")
	}
	if _, err := workload.ParseThreadMode(string(c.Mode)); err != nil {
		return err
	}
	if _, err := workload.ParseSyncMode(string(c.Sync)); err != nil {
		return err
	}
	if c.MemoryMiB < 0 || c.WorkMillions < 0 {
		return errors.New("memory and work cannot be negative")
	}
	if c.Limits.CPUPercent < 0 || c.Limits.MemoryBytes < 0 {
		return errors.New("limits cannot be negative")
	}
	if c.Timeout < 0 {
		return errors.New("timeout cannot be negative")
	}
	return nil
}

// WorkerArgs returns the flags that make the worker run this test.
func (c Config) WorkerArgs() []string {
	args := []string{
		"-threads", strconv.Itoa(c.Threads),
		"-mode", string(c.Mode),
		"-mem", strconv.Itoa(c.MemoryMiB),
		"-work", strconv.FormatInt(c.WorkMillions, 10),
	}
	if c.Sync != "" && c.Sync != workload.SyncOff {
		args = append(args, "-sync", string(c.Sync))
	}
	return args
}
