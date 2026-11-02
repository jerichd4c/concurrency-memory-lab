// Command resource-lab runs controlled concurrency and memory experiments.
// For now it only has the worker mode, which is the test load itself.
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "worker" {
		os.Exit(runWorker(os.Args[2:]))
	}
	fmt.Fprintln(os.Stderr, "usage: resource-lab worker [flags]  (resource-lab worker -h for help)")
	os.Exit(exitUsage)
}
