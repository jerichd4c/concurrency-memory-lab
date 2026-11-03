// Command resource-lab runs controlled concurrency and memory experiments.
// For now it has three modes: run, which runs a complete test with limits;
// worker, the test load itself; and inspect, which prints the measurements
// of a running process.
package main

import (
	"fmt"
	"os"
)

const usage = `usage:
  resource-lab run [flags]      run a test with CPU and memory limits (needs root for the limits)
  resource-lab worker [flags]   run the test load (-h for its flags)
  resource-lab inspect <pid>    print the measurements of a process`

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "run":
			os.Exit(runRun(os.Args[2:]))
		case "worker":
			os.Exit(runWorker(os.Args[2:]))
		case "inspect":
			os.Exit(runInspect(os.Args[2:]))
		}
	}
	fmt.Fprintln(os.Stderr, usage)
	os.Exit(exitUsage)
}
