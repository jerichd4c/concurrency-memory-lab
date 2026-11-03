package runner

import (
	"strconv"
	"strings"
	"syscall"
	"time"

	"concurrency-memory-lab/cgroup"
	"concurrency-memory-lab/metrics"
	"concurrency-memory-lab/workload"
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

	Work       Work   // what the worker reported about its CPU phase
	Output     string // what the worker printed
	CleanupErr error  // the cgroup could not be removed
}

// Work is the CPU phase as reported by the worker.
type Work struct {
	Reported bool          // the worker printed its CPU line
	Shared   bool          // shared counter test instead of independent work
	Done     int64         // iterations or increments done
	Counter  int64         // final value of the shared counter
	Elapsed  time.Duration // time of the CPU phase alone, without the memory phase
}

// Lost returns how many counter updates were lost.
func (w Work) Lost() int64 {
	if !w.Shared {
		return 0
	}
	return w.Done - w.Counter
}

// parseWork reads the CPU line of the worker output, one of:
//
//	cpu work: N iterations in T (checksum X)
//	shared counter (MODE): C of N increments, lost L (P%) in T
func parseWork(output string) Work {
	for line := range strings.Lines(output) {
		f := strings.Fields(line)
		var w Work
		var done, counter, elapsed string
		switch {
		case len(f) >= 6 && f[0] == "cpu" && f[1] == "work:":
			done, elapsed = f[2], f[5]
		case len(f) >= 12 && f[0] == "shared" && f[1] == "counter":
			w.Shared = true
			counter, done, elapsed = f[3], f[5], f[11]
		default:
			continue
		}
		var err error
		if w.Done, err = strconv.ParseInt(done, 10, 64); err != nil {
			return Work{}
		}
		if w.Shared {
			if w.Counter, err = strconv.ParseInt(counter, 10, 64); err != nil {
				return Work{}
			}
		}
		if w.Elapsed, err = time.ParseDuration(elapsed); err != nil {
			return Work{}
		}
		w.Reported = true
		return w
	}
	return Work{}
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
