package cgroup

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// CleanOrphans removes the groups left behind by a controller that died
// without cleaning up, for example killed with SIGKILL. Groups are named
// run-<controller pid>-<n>; those whose controller is still alive belong
// to a running program and are left alone. It returns how many groups it
// removed.
func CleanOrphans(parent string) (int, error) {
	entries, err := os.ReadDir(parent)
	if errors.Is(err, fs.ErrNotExist) {
		return 0, nil // nothing was ever created
	}
	if err != nil {
		return 0, err
	}
	removed := 0
	var errs []error
	for _, e := range entries {
		pid, ok := ownerPID(e.Name())
		if !e.IsDir() || !ok || alive(pid) {
			continue
		}
		g := &Group{Dir: filepath.Join(parent, e.Name())}
		g.Kill() // a worker could still be inside
		if err := g.Remove(); err != nil {
			errs = append(errs, err)
			continue
		}
		removed++
	}
	return removed, errors.Join(errs...)
}

// RemoveParent deletes the parent group when no test group is left in it,
// so the program leaves /sys/fs/cgroup as it found it. A parent that still
// has groups is kept.
func RemoveParent(parent string) {
	os.Remove(parent)
}

// ownerPID reads the controller PID from a name like "run-1234-1".
func ownerPID(name string) (int, bool) {
	rest, ok := strings.CutPrefix(name, "run-")
	if !ok {
		return 0, false
	}
	pidText, _, ok := strings.Cut(rest, "-")
	if !ok {
		return 0, false
	}
	pid, err := strconv.Atoi(pidText)
	return pid, err == nil && pid > 0
}

// alive reports whether a process exists. Signal 0 checks without sending
// anything; EPERM means it exists but belongs to someone else.
func alive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}
