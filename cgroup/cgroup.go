// Package cgroup creates the cgroup v2 group where the test load runs,
// applies its CPU and memory limits and removes it afterwards.
package cgroup

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"concurrency-memory-lab/metrics"
)

// DefaultParent groups every cgroup created by the application.
var DefaultParent = filepath.Join(metrics.CgroupRoot, "concurrency-memory-lab")

// Group is one cgroup directory, created for a single test run.
type Group struct {
	Dir string
}

// Available reports whether cgroup v2 is mounted and the root cgroup lets
// its children use the cpu and memory controllers.
func Available() error {
	data, err := os.ReadFile(filepath.Join(metrics.CgroupRoot, "cgroup.subtree_control"))
	if errors.Is(err, fs.ErrNotExist) {
		return errors.New("cgroup v2 is not mounted at " + metrics.CgroupRoot)
	}
	if err != nil {
		return err
	}
	enabled := strings.Fields(string(data))
	for _, c := range []string{"cpu", "memory"} {
		if !slices.Contains(enabled, c) {
			return fmt.Errorf("the %s controller is not enabled in %s", c, metrics.CgroupRoot)
		}
	}
	return nil
}

// New creates the cgroup parent/name. The parent is created when needed and
// enables the cpu and memory controllers for its children. Writing there
// needs root.
func New(parent, name string) (*Group, error) {
	if err := os.Mkdir(parent, 0o755); err != nil && !errors.Is(err, fs.ErrExist) {
		return nil, explain(err)
	}
	control := filepath.Join(parent, "cgroup.subtree_control")
	if err := os.WriteFile(control, []byte("+cpu +memory"), 0o644); err != nil {
		return nil, fmt.Errorf("enable cpu and memory controllers: %w", explain(err))
	}
	dir := filepath.Join(parent, name)
	if err := os.Mkdir(dir, 0o755); err != nil {
		return nil, explain(err)
	}
	return &Group{Dir: dir}, nil
}

// Open returns the directory of the group as a file. Its descriptor is what
// the kernel needs to start a process directly inside the group.
func (g *Group) Open() (*os.File, error) {
	return os.Open(g.Dir)
}

// Kill sends SIGKILL to every process in the group at once. The file is
// opened without O_CREATE: a directory that is not a cgroup gets no stray
// file.
func (g *Group) Kill() error {
	f, err := os.OpenFile(filepath.Join(g.Dir, "cgroup.kill"), os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString("1")
	return err
}

// Remove deletes the group. The kernel refuses (EBUSY) while it still has
// processes, so it retries for a moment after they were killed.
func (g *Group) Remove() error {
	var err error
	for range 50 {
		err = os.Remove(g.Dir)
		if err == nil || errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if !errors.Is(err, syscall.EBUSY) {
			return err
		}
		time.Sleep(20 * time.Millisecond)
	}
	return err
}

// explain adds a hint to permission errors.
func explain(err error) error {
	if errors.Is(err, fs.ErrPermission) {
		return fmt.Errorf("%w (run the application as root, for example with sudo)", err)
	}
	return err
}
