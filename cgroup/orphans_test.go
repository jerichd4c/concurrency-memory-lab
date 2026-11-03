package cgroup

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// deadPID returns the PID of a process that has already exited.
func deadPID(t *testing.T) int {
	t.Helper()
	cmd := exec.Command("true")
	if err := cmd.Run(); err != nil {
		t.Skip("cannot run true:", err)
	}
	return cmd.Process.Pid
}

func TestCleanOrphans(t *testing.T) {
	parent := t.TempDir()
	dead := deadPID(t)
	dirs := map[string]bool{ // name: must be kept
		fmt.Sprintf("run-%d-1", dead):        false, // controller gone: orphan
		fmt.Sprintf("run-%d-2", dead):        false,
		fmt.Sprintf("run-%d-1", os.Getpid()): true, // controller alive (us)
		"run-abc-1":                          true, // not our name format
		"other":                              true,
	}
	for name := range dirs {
		if err := os.Mkdir(filepath.Join(parent, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// A file is never touched, even with a matching name.
	file := filepath.Join(parent, fmt.Sprintf("run-%d-9", dead))
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	n, err := CleanOrphans(parent)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("removed %d groups, want 2", n)
	}
	for name, keep := range dirs {
		_, err := os.Stat(filepath.Join(parent, name))
		if exists := err == nil; exists != keep {
			t.Errorf("%s: exists %v, want %v", name, exists, keep)
		}
	}
	if _, err := os.Stat(file); err != nil {
		t.Error("the file was removed")
	}
}

func TestCleanOrphansWithoutParent(t *testing.T) {
	n, err := CleanOrphans(filepath.Join(t.TempDir(), "missing"))
	if n != 0 || err != nil {
		t.Errorf("missing parent: %d, %v; want 0, nil", n, err)
	}
}

func TestRemoveParentKeepsAParentWithGroups(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "concurrency-memory-lab")
	if err := os.MkdirAll(filepath.Join(parent, "run-1-1"), 0o755); err != nil {
		t.Fatal(err)
	}
	RemoveParent(parent)
	if _, err := os.Stat(parent); err != nil {
		t.Error("a parent with groups was removed")
	}
	os.Remove(filepath.Join(parent, "run-1-1"))
	RemoveParent(parent)
	if _, err := os.Stat(parent); err == nil {
		t.Error("an empty parent was kept")
	}
}

func TestOwnerPID(t *testing.T) {
	tests := map[string]int{"run-1234-1": 1234, "run-7-12": 7, "run-0-1": 0, "run-x-1": 0, "run-1234": 0, "job-1-1": 0}
	for name, want := range tests {
		pid, ok := ownerPID(name)
		if ok != (want > 0) || pid != want && ok {
			t.Errorf("ownerPID(%q) = %d, %v; want %d", name, pid, ok, want)
		}
	}
}

func TestAlive(t *testing.T) {
	if !alive(os.Getpid()) {
		t.Error("this process is not alive")
	}
	if alive(deadPID(t)) {
		t.Error("an exited process is alive")
	}
}

func TestKillDoesNotCreateFiles(t *testing.T) {
	g := &Group{Dir: t.TempDir()}
	if err := g.Kill(); err == nil {
		t.Error("Kill in a plain directory returned no error")
	}
	if _, err := os.Stat(filepath.Join(g.Dir, "cgroup.kill")); err == nil {
		t.Error("Kill created cgroup.kill")
	}
}
