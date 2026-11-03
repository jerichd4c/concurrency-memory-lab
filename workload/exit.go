package workload

// Exit codes of the worker process. They are shared by the worker, which
// returns them, and the runner, which reads them to tell how a test ended.
const (
	ExitOK      = 0
	ExitError   = 1
	ExitUsage   = 2
	ExitStopped = 3 // stopped cleanly after SIGTERM or SIGINT
)
