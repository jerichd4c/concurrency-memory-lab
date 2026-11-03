package runner

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync/atomic"
	"syscall"
	"time"

	"resource-lab/cgroup"
	"resource-lab/metrics"
)

// stopGrace is how long the worker has to stop after SIGTERM before it is
// killed.
const stopGrace = 2 * time.Second

// ErrFinished is returned by Sample once the worker has exited.
var ErrFinished = errors.New("the worker has finished")

var runCount atomic.Int64

// Sample is one reading of the running worker.
type Sample struct {
	Elapsed   time.Duration
	Process   metrics.Process
	HasCgroup bool
	Cgroup    metrics.Cgroup
}

// Run is a worker process that is running or has finished.
type Run struct {
	cfg      Config
	cmd      *exec.Cmd
	group    *cgroup.Group // nil when the cgroup could not be created
	groupErr error
	limits   []cgroup.LimitResult
	start    time.Time
	timer    *time.Timer
	reason   atomic.Int32
	peakSwap atomic.Uint64
	output   bytes.Buffer
	done     chan struct{}
	result   Result
}

// Start creates the cgroup, applies the limits and starts exe (this same
// program) in worker mode directly inside the cgroup. If the cgroup cannot
// be created the test still runs, but every limit is reported as not
// applied.
func Start(exe string, cfg Config) (*Run, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	r := &Run{cfg: cfg, done: make(chan struct{})}

	name := fmt.Sprintf("run-%d-%d", os.Getpid(), runCount.Add(1))
	r.group, r.groupErr = newGroup(name)
	if r.group != nil {
		r.limits = r.group.Apply(cfg.Limits)
	} else {
		r.limits = cfg.Limits.NotApplied(r.groupErr)
	}

	cmd := exec.Command(exe, append([]string{"worker"}, cfg.WorkerArgs()...)...)
	cmd.Stdout = &r.output
	cmd.Stderr = &r.output
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid:   true,            // own process group: Ctrl+C in the terminal reaches only us
		Pdeathsig: syscall.SIGKILL, // the worker dies if the controller dies
	}
	if r.group != nil {
		dir, err := r.group.Open()
		if err != nil {
			r.group.Remove()
			return nil, err
		}
		defer dir.Close()
		// clone3 with CLONE_INTO_CGROUP: the child is born inside the
		// group, so not a single page escapes the limits.
		cmd.SysProcAttr.UseCgroupFD = true
		cmd.SysProcAttr.CgroupFD = int(dir.Fd())
	}

	r.start = time.Now()
	if err := cmd.Start(); err != nil {
		if r.group != nil {
			r.group.Remove()
		}
		return nil, err
	}
	r.cmd = cmd
	if cfg.Timeout > 0 {
		r.timer = time.AfterFunc(cfg.Timeout, func() { r.stop(reasonTimeout) })
	}
	go r.wait()
	return r, nil
}

func newGroup(name string) (*cgroup.Group, error) {
	if err := cgroup.Available(); err != nil {
		return nil, err
	}
	return cgroup.New(cgroup.DefaultParent, name)
}

// Pid returns the process ID of the worker.
func (r *Run) Pid() int { return r.cmd.Process.Pid }

// CgroupDir returns the cgroup of the run, or "" if it has none.
func (r *Run) CgroupDir() string {
	if r.group == nil {
		return ""
	}
	return r.group.Dir
}

// CgroupErr returns why the run has no cgroup.
func (r *Run) CgroupErr() error { return r.groupErr }

// Limits returns which requested limits are in force.
func (r *Run) Limits() []cgroup.LimitResult { return r.limits }

// Done is closed when the worker has exited and the result is ready.
func (r *Run) Done() <-chan struct{} { return r.done }

// Result waits for the worker to exit and returns the result.
func (r *Run) Result() Result {
	<-r.done
	return r.result
}

// Sample reads the current measurements of the worker and its cgroup.
func (r *Run) Sample() (Sample, error) {
	select {
	case <-r.done:
		return Sample{}, ErrFinished
	default:
	}
	s := Sample{Elapsed: time.Since(r.start)}
	p, err := metrics.ReadProcess(r.Pid())
	if err != nil {
		return s, err
	}
	s.Process = p
	for {
		old := r.peakSwap.Load()
		if p.SwapBytes <= old || r.peakSwap.CompareAndSwap(old, p.SwapBytes) {
			break
		}
	}
	if r.group != nil {
		if c, err := metrics.ReadCgroup(r.group.Dir); err == nil {
			s.Cgroup, s.HasCgroup = c, true
		}
	}
	return s, nil
}

// Stop asks the worker to finish: SIGTERM first, and SIGKILL to the whole
// cgroup if it is still running after stopGrace.
func (r *Run) Stop() { r.stop(reasonUser) }

func (r *Run) stop(reason stopReason) {
	if !r.reason.CompareAndSwap(int32(reasonNone), int32(reason)) {
		return // already stopping
	}
	r.cmd.Process.Signal(syscall.SIGTERM)
	time.AfterFunc(stopGrace, func() {
		select {
		case <-r.done:
		default:
			if r.group != nil {
				r.group.Kill()
			}
			r.cmd.Process.Kill()
		}
	})
}

// wait reaps the worker, collects the result and removes the cgroup.
func (r *Run) wait() {
	r.cmd.Wait()
	end := time.Now()
	if r.timer != nil {
		r.timer.Stop()
	}

	res := Result{
		Config:        r.cfg,
		Limits:        r.limits,
		Duration:      end.Sub(r.start),
		PeakSwapBytes: r.peakSwap.Load(),
		Output:        r.output.String(),
		ExitCode:      -1,
	}
	res.Work = parseWork(res.Output)
	if st := r.cmd.ProcessState; st != nil {
		res.ExitCode = st.ExitCode()
		if ws, ok := st.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
			res.Signal = ws.Signal()
		}
		if ru, ok := st.SysUsage().(*syscall.Rusage); ok {
			res.PeakResidentBytes = uint64(ru.Maxrss) * 1024 // Maxrss is in kB
			res.MinorFaults = uint64(ru.Minflt)
			res.MajorFaults = uint64(ru.Majflt)
			res.UserTime = time.Duration(ru.Utime.Nano())
			res.SystemTime = time.Duration(ru.Stime.Nano())
		}
	}
	if r.group != nil {
		if c, err := metrics.ReadCgroup(r.group.Dir); err == nil {
			res.Cgroup, res.HasCgroup = c, true
		}
		r.group.Kill() // anything the worker may have left behind
		res.CleanupErr = r.group.Remove()
	}
	res.Outcome = classify(res.ExitCode, res.Signal, stopReason(r.reason.Load()), res.Cgroup.OOMKills)

	r.result = res
	close(r.done)
}
