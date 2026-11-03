// Command concurrency-memory-lab runs controlled concurrency and memory
// experiments. Without arguments it opens the terminal interface. The other
// modes are: run, which runs one test from the command line; worker, the
// test load itself; and inspect, which prints the measurements of a running
// process.
package main

import (
	"fmt"
	"os"

	"concurrency-memory-lab/ui"
)

const usage = `usage:
  concurrency-memory-lab                  open the interface (run it with sudo to apply limits)
  concurrency-memory-lab run [flags]      run one test from the command line
  concurrency-memory-lab worker [flags]   run the test load (-h for its flags)
  concurrency-memory-lab inspect <pid>    print the measurements of a process`

func main() {
	if len(os.Args) == 1 {
		os.Exit(runUI())
	}
	switch os.Args[1] {
	case "run":
		os.Exit(runRun(os.Args[2:]))
	case "worker":
		os.Exit(runWorker(os.Args[2:]))
	case "inspect":
		os.Exit(runInspect(os.Args[2:]))
	}
	fmt.Fprintln(os.Stderr, usage)
	os.Exit(exitUsage)
}

func runUI() int {
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot find this program:", err)
		return exitError
	}
	if err := ui.Run(exe); err != nil {
		fmt.Fprintln(os.Stderr, "interface:", err)
		return exitError
	}
	return exitOK
}
