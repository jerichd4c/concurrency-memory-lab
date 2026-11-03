package main

import (
	"fmt"
	"os"
	"strconv"

	"concurrency-memory-lab/metrics"
)

// runInspect prints the measurements of a running process, its cgroup and
// the machine. It is a debugging aid; the TUI will show the same values.
func runInspect(args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: concurrency-memory-lab inspect <pid>")
		return exitUsage
	}
	pid, err := strconv.Atoi(args[0])
	if err != nil || pid <= 0 {
		fmt.Fprintf(os.Stderr, "invalid pid %q\n", args[0])
		return exitUsage
	}

	p, err := metrics.ReadProcess(pid)
	if err != nil {
		fmt.Fprintln(os.Stderr, "process:", err)
		return exitError
	}
	fmt.Printf("process %d\n", pid)
	fmt.Printf("  virtual memory   %s\n", mib(p.VirtualBytes))
	fmt.Printf("  resident (RAM)   %s  (peak %s)\n", mib(p.ResidentBytes), mib(p.PeakResidentBytes))
	fmt.Printf("  swap             %s\n", mib(p.SwapBytes))
	fmt.Printf("  page faults      %d minor, %d major\n", p.MinorFaults, p.MajorFaults)
	fmt.Printf("  threads          %d\n", p.Threads)
	fmt.Printf("  cpu time         %v\n", p.CPUTime)

	dir, err := metrics.CgroupOf(pid)
	if err != nil {
		fmt.Println("cgroup: unknown:", err)
	} else if c, err := metrics.ReadCgroup(dir); err != nil {
		fmt.Printf("cgroup %s: not readable: %v\n", dir, err)
	} else {
		fmt.Printf("cgroup %s\n", dir)
		fmt.Printf("  memory           %s  (peak %s)\n", mib(c.MemoryBytes), mib(c.MemoryPeakBytes))
		if c.SwapAvailable {
			fmt.Printf("  swap             %s  (peak %s)\n", mib(c.SwapBytes), mib(c.SwapPeakBytes))
		} else {
			fmt.Println("  swap             not accounted")
		}
		fmt.Printf("  oom kills        %d\n", c.OOMKills)
		fmt.Printf("  cpu usage        %v\n", c.CPUUsage)
		fmt.Printf("  throttled        %d of %d periods, %v\n", c.ThrottledPeriods, c.Periods, c.Throttled)
	}

	if s, err := metrics.ReadSystem(); err == nil {
		fmt.Println("system")
		fmt.Printf("  RAM              %s available of %s\n", mib(s.MemAvailableBytes), mib(s.MemTotalBytes))
		fmt.Printf("  swap             %s free of %s\n", mib(s.SwapFreeBytes), mib(s.SwapTotalBytes))
	}
	return exitOK
}

// mib formats a number of bytes in MiB.
func mib(b uint64) string {
	return fmt.Sprintf("%.1f MiB", float64(b)/(1<<20))
}
