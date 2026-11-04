// Command concurrency-memory-lab runs controlled concurrency and memory
// experiments. Without arguments (or with only -lang) it opens the terminal
// interface. The other modes are: run, which runs one test from the command
// line; worker, the test load itself; and inspect, which prints the
// measurements of a running process.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"concurrency-memory-lab/ui"
)

const usage = `usage:
  concurrency-memory-lab [-lang en|es]    open the interface (run it with sudo to apply limits)
  concurrency-memory-lab run [flags]      run one test from the command line
  concurrency-memory-lab worker [flags]   run the test load (-h for its flags)
  concurrency-memory-lab inspect <pid>    print the measurements of a process`

func main() {
	if len(os.Args) == 1 || strings.HasPrefix(os.Args[1], "-") {
		os.Exit(runUI(os.Args[1:]))
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

// runUI opens the interface. Its only flag is -lang; without it the
// language comes from the locale (LANG), and the l key changes it anyway.
func runUI(args []string) int {
	fs := flag.NewFlagSet("concurrency-memory-lab", flag.ContinueOnError)
	langName := fs.String("lang", "", "language of the interface: en or es (default: from LANG)")
	fs.Usage = func() { fmt.Fprintln(os.Stderr, usage) }
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	if fs.NArg() > 0 {
		fmt.Fprintln(os.Stderr, usage)
		return exitUsage
	}
	lang, err := ui.ParseLang(*langName)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return exitUsage
	}

	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot find this program:", err)
		return exitError
	}
	if err := ui.Run(exe, lang); err != nil {
		fmt.Fprintln(os.Stderr, "interface:", err)
		return exitError
	}
	return exitOK
}
