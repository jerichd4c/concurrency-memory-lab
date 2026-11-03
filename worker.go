package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"sync/atomic"
	"syscall"
	"time"

	"concurrency-memory-lab/workload"
)

// Exit codes of the program, shared with the runner.
const (
	exitOK      = workload.ExitOK
	exitError   = workload.ExitError
	exitUsage   = workload.ExitUsage
	exitStopped = workload.ExitStopped
)

// runWorker is the test load: it reserves and touches memory, then runs CPU
// work across several workers: independent work, or increments of one shared
// counter. SIGTERM or SIGINT stops it cleanly.
func runWorker(args []string) int {
	fs := flag.NewFlagSet("worker", flag.ContinueOnError)
	threads := fs.Int("threads", 4, "number of workers")
	modeName := fs.String("mode", string(workload.Goroutines), "worker type: goroutine or os-thread")
	memMiB := fs.Int("mem", 64, "memory to reserve and touch, in MiB (0 = none)")
	work := fs.Int64("work", 2000, "CPU work, in millions of iterations or increments (0 = none)")
	syncName := fs.String("sync", string(workload.SyncOff), "shared counter: off, none, mutex or atomic")
	pause := fs.Duration("pause", 0, "pause after each memory phase, to inspect /proc/<pid>")
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	mode, err := workload.ParseThreadMode(*modeName)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return exitUsage
	}
	syncMode, err := workload.ParseSyncMode(*syncName)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return exitUsage
	}
	if *threads < 1 || *memMiB < 0 || *work < 0 {
		fmt.Fprintln(os.Stderr, "threads must be >= 1; mem and work must be >= 0")
		return exitUsage
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()
	var stop atomic.Bool
	go func() {
		<-ctx.Done()
		stop.Store(true)
	}()

	start := time.Now()
	fmt.Printf("pid=%d mode=%s threads=%d GOMAXPROCS=%d CPUs=%d\n",
		os.Getpid(), mode, *threads, runtime.GOMAXPROCS(0), runtime.NumCPU())

	if *memMiB > 0 {
		region, err := workload.Reserve(*memMiB << 20)
		if err != nil {
			fmt.Fprintln(os.Stderr, "reserve:", err)
			return exitError
		}
		defer region.Release()
		fmt.Printf("reserved %d MiB: virtual memory grows, RSS does not\n", *memMiB)
		wait(ctx, *pause)

		t := time.Now()
		pages := region.Touch(&stop)
		fmt.Printf("touched %d pages in %v: RSS grows, one page fault per page\n",
			pages, time.Since(t).Round(time.Millisecond))
		wait(ctx, *pause)
	}

	if *work > 0 && !stop.Load() {
		t := time.Now()
		if syncMode == workload.SyncOff {
			done, sum := workload.RunCPU(*work*1_000_000, *threads, mode, &stop)
			fmt.Printf("cpu work: %d iterations in %v (checksum %016x)\n",
				done, time.Since(t).Round(time.Millisecond), sum)
		} else {
			done, counter := workload.RunCounter(*work*1_000_000, *threads, mode, syncMode, &stop)
			lost := done - counter
			fmt.Printf("shared counter (%s): %d of %d increments, lost %d (%.1f%%) in %v\n",
				syncMode, counter, done, lost, percent(lost, done), time.Since(t).Round(time.Millisecond))
		}
	}

	fmt.Printf("total %v\n", time.Since(start).Round(time.Millisecond))
	if stop.Load() {
		fmt.Println("result: stopped")
		return exitStopped
	}
	fmt.Println("result: ok")
	return exitOK
}

// percent returns part/whole as a percentage, 0 when whole is 0.
func percent(part, whole int64) float64 {
	if whole == 0 {
		return 0
	}
	return 100 * float64(part) / float64(whole)
}

// wait sleeps for d, or less if the worker is asked to stop.
func wait(ctx context.Context, d time.Duration) {
	if d <= 0 {
		return
	}
	select {
	case <-time.After(d):
	case <-ctx.Done():
	}
}
