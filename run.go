package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"resource-lab/cgroup"
	"resource-lab/runner"
	"resource-lab/workload"
)

// runRun runs one complete test from the command line: it starts the worker
// with the requested limits, prints a sample every interval and the result
// at the end. Ctrl+C stops the test. The TUI will use the same runner.
func runRun(args []string) int {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	threads := fs.Int("threads", 4, "number of workers")
	modeName := fs.String("mode", string(workload.Goroutines), "worker type: goroutine or os-thread")
	memMiB := fs.Int("mem", 64, "memory the worker reserves and touches, in MiB")
	work := fs.Int64("work", 2000, "CPU work, in millions of iterations")
	cpu := fs.Int("cpu", 0, "CPU limit, in percent of one CPU (0 = no limit, 200 = two CPUs)")
	memLimit := fs.Int64("mem-limit", 0, "memory limit, in MiB (0 = no limit)")
	swapLimit := fs.Int64("swap-limit", -1, "swap limit, in MiB (-1 = no limit, 0 = no swap)")
	timeout := fs.Duration("timeout", time.Minute, "stop the test after this time (0 = never)")
	interval := fs.Duration("interval", 500*time.Millisecond, "time between samples")
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	if *interval <= 0 {
		fmt.Fprintln(os.Stderr, "interval must be positive")
		return exitUsage
	}

	swapBytes := *swapLimit << 20
	if *swapLimit < 0 {
		swapBytes = -1
	}
	cfg := runner.Config{
		Threads:      *threads,
		Mode:         workload.ThreadMode(*modeName),
		MemoryMiB:    *memMiB,
		WorkMillions: *work,
		Limits: cgroup.Limits{
			CPUPercent:  *cpu,
			MemoryBytes: *memLimit << 20,
			SwapBytes:   swapBytes,
		},
		Timeout: *timeout,
	}
	if err := cfg.Validate(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return exitUsage
	}

	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot find this program:", err)
		return exitError
	}
	r, err := runner.Start(exe, cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "start:", err)
		return exitError
	}

	fmt.Printf("worker pid %d\n", r.Pid())
	if dir := r.CgroupDir(); dir != "" {
		fmt.Printf("cgroup %s\n", dir)
	} else {
		fmt.Printf("cgroup: none (%v)\n", r.CgroupErr())
	}
	printLimits(r.Limits())

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigs)
	tick := time.NewTicker(*interval)
	defer tick.Stop()

	for running := true; running; {
		select {
		case <-tick.C:
			if s, err := r.Sample(); err == nil {
				printSample(s)
			}
		case <-sigs:
			fmt.Println("stopping...")
			r.Stop()
		case <-r.Done():
			running = false
		}
	}
	printResult(r.Result())
	return exitOK
}

func printLimits(limits []cgroup.LimitResult) {
	if len(limits) == 0 {
		fmt.Println("limits: none requested")
		return
	}
	for _, l := range limits {
		fmt.Println("limit", l)
	}
	for _, l := range limits {
		if !l.Applied {
			fmt.Println("WARNING: this test is NOT limited as requested")
			return
		}
	}
}

func printSample(s runner.Sample) {
	p := s.Process
	line := fmt.Sprintf("%6.1fs  virt %s  rss %s  swap %s  faults %d minor %d major  threads %d  cpu %v",
		s.Elapsed.Seconds(), mib(p.VirtualBytes), mib(p.ResidentBytes), mib(p.SwapBytes),
		p.MinorFaults, p.MajorFaults, p.Threads, p.CPUTime.Round(time.Millisecond))
	if s.HasCgroup {
		c := s.Cgroup
		line += fmt.Sprintf("  | cgroup mem %s  throttled %d/%d", mib(c.MemoryBytes), c.ThrottledPeriods, c.Periods)
	}
	fmt.Println(line)
}

func printResult(res runner.Result) {
	fmt.Println("---- result")
	fmt.Printf("outcome          %s\n", res.Outcome)
	if res.Signal != 0 {
		fmt.Printf("exit             signal %d (%v)\n", int(res.Signal), res.Signal)
	} else {
		fmt.Printf("exit             code %d\n", res.ExitCode)
	}
	fmt.Printf("duration         %v\n", res.Duration.Round(time.Millisecond))
	fmt.Printf("cpu time         %v user, %v system\n",
		res.UserTime.Round(time.Millisecond), res.SystemTime.Round(time.Millisecond))
	fmt.Printf("peak resident    %s\n", mib(res.PeakResidentBytes))
	fmt.Printf("page faults      %d minor, %d major\n", res.MinorFaults, res.MajorFaults)
	if res.HasCgroup {
		c := res.Cgroup
		fmt.Printf("cgroup memory    peak %s\n", mib(c.MemoryPeakBytes))
		if c.SwapAvailable {
			fmt.Printf("cgroup swap      peak %s\n", mib(c.SwapPeakBytes))
		} else {
			fmt.Println("cgroup swap      not accounted")
		}
		fmt.Printf("oom kills        %d\n", c.OOMKills)
		fmt.Printf("cpu throttled    %d of %d periods, %v\n",
			c.ThrottledPeriods, c.Periods, c.Throttled.Round(time.Millisecond))
	} else {
		fmt.Printf("swap (sampled)   peak %s\n", mib(res.PeakSwapBytes))
	}
	printLimits(res.Limits)
	if res.CleanupErr != nil {
		fmt.Println("WARNING: cgroup not removed:", res.CleanupErr)
	}
	fmt.Println("worker output:")
	for line := range strings.Lines(res.Output) {
		fmt.Print("  ", line)
	}
}
