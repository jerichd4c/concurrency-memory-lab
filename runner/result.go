package runner

import (
	"syscall"
	"time"

	"resource-lab/cgroup"
	"resource-lab/metrics"
	"resource-lab/workload"
)

// Outcome is how a test run ended.
type Outcome string

const (
	OutcomeCompleted Outcome = "completed"
	OutcomeStopped   Outcome = "stopped by the user"
	OutcomeTimeout   Outcome = "stopped: time limit reached"
	OutcomeOOM       Outcome = "killed by the kernel: out of memory in the cgroup"
	OutcomeKilled    Outcome = "killed by a signal"
	OutcomeFailed    Outcome = "failed"
)

// Result is everything known about a finished run.
type Result struct {
	Config   Config
	Limits   []cgroup.LimitResult
	Outcome  Outcome
	ExitCode int            // -1 when the worker was killed by a signal
	Signal   syscall.Signal // the signal that killed it, if any
	Duration time.Duration  // wall-clock time from start to exit

	// From the kernel's accounting of the finished process (getrusage).
	PeakResidentBytes uint64
	MinorFaults       uint64
	MajorFaults       uint64
	UserTime          time.Duration
	SystemTime        time.Duration

	// Final values of the cgroup, when the run had one.
	HasCgroup bool
	Cgroup    metrics.Cgroup
	// Highest VmSwap seen while sampling; the cgroup's swap peak is more
	// precise when HasCgroup is true.
	PeakSwapBytes uint64

	Output     string // what the worker printed
	CleanupErr error  // the cgroup could not be removed
}

// stopReason records why the runner itself stopped the worker.
type stopReason int32

const (
	reasonNone stopReason = iota
	reasonUser
	reasonTimeout
)

// classify decides the outcome from how the process ended. The order
// matters: a worker that exits 0 completed even if a stop arrived late, and
// a SIGKILL counts as out of memory only if the cgroup recorded an OOM kill.
func classify(exitCode int, signal syscall.Signal, reason stopReason, oomKills uint64) Outcome {
	switch {
	case exitCode == workload.ExitOK:
		return OutcomeCompleted
	case signal == syscall.SIGKILL && oomKills > 0:
		return OutcomeOOM
	case reason == reasonTimeout:
		return OutcomeTimeout
	case reason == reasonUser, exitCode == workload.ExitStopped:
		return OutcomeStopped
	case signal != 0:
		return OutcomeKilled
	}
	return OutcomeFailed
}
