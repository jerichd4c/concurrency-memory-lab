package metrics

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// CgroupRoot is where the cgroup v2 hierarchy is mounted.
const CgroupRoot = "/sys/fs/cgroup"

// Cgroup holds the measurements of a cgroup v2 directory. They cover every
// process inside it and include memory the kernel charges to the cgroup
// (kernel structures, page cache), so they can be larger than VmRSS.
type Cgroup struct {
	MemoryBytes      uint64        // memory.current: RAM charged now
	MemoryPeakBytes  uint64        // memory.peak: highest memory.current
	SwapAvailable    bool          // memory.swap.* exist (swap accounting on)
	SwapBytes        uint64        // memory.swap.current
	SwapPeakBytes    uint64        // memory.swap.peak
	OOMKills         uint64        // memory.events oom_kill
	CPUUsage         time.Duration // cpu.stat usage_usec
	Periods          uint64        // cpu.stat nr_periods: quota periods elapsed
	ThrottledPeriods uint64        // cpu.stat nr_throttled: periods that ran out of quota
	Throttled        time.Duration // cpu.stat throttled_usec: time spent stopped
}

// CgroupOf returns the cgroup directory of a process, from the "0::<path>"
// line of /proc/<pid>/cgroup.
func CgroupOf(pid int) (string, error) {
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/cgroup")
	if err != nil {
		return "", err
	}
	for line := range strings.Lines(string(data)) {
		if path, ok := strings.CutPrefix(strings.TrimSpace(line), "0::"); ok {
			return filepath.Join(CgroupRoot, path), nil
		}
	}
	return "", errors.New("no cgroup v2 entry in /proc/" + strconv.Itoa(pid) + "/cgroup")
}

// ReadCgroup reads the memory and CPU files of a cgroup directory. A missing
// swap file is not an error: it means swap is not accounted for.
func ReadCgroup(dir string) (Cgroup, error) {
	var c Cgroup
	var err error
	if c.MemoryBytes, err = readUint(dir, "memory.current"); err != nil {
		return Cgroup{}, err
	}
	if c.MemoryPeakBytes, err = readUint(dir, "memory.peak"); err != nil {
		return Cgroup{}, err
	}
	events, err := readKeyed(dir, "memory.events")
	if err != nil {
		return Cgroup{}, err
	}
	c.OOMKills = events["oom_kill"]

	cpu, err := readKeyed(dir, "cpu.stat")
	if err != nil {
		return Cgroup{}, err
	}
	c.CPUUsage = time.Duration(cpu["usage_usec"]) * time.Microsecond
	c.Periods = cpu["nr_periods"]
	c.ThrottledPeriods = cpu["nr_throttled"]
	c.Throttled = time.Duration(cpu["throttled_usec"]) * time.Microsecond

	c.SwapBytes, err = readUint(dir, "memory.swap.current")
	switch {
	case err == nil:
		c.SwapAvailable = true
		if c.SwapPeakBytes, err = readUint(dir, "memory.swap.peak"); err != nil {
			return Cgroup{}, err
		}
	case !errors.Is(err, fs.ErrNotExist):
		return Cgroup{}, err
	}
	return c, nil
}

// readUint reads a cgroup file that holds a single number.
func readUint(dir, name string) (uint64, error) {
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return 0, err
	}
	v, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", name, err)
	}
	return v, nil
}

// readKeyed reads a cgroup file of "key value" lines.
func readKeyed(dir, name string) (map[string]uint64, error) {
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return nil, err
	}
	values, err := parseKeyed(string(data))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return values, nil
}

// parseKeyed parses "key value" lines, the format of cpu.stat and
// memory.events.
func parseKeyed(text string) (map[string]uint64, error) {
	values := make(map[string]uint64)
	for line := range strings.Lines(text) {
		key, value, ok := strings.Cut(strings.TrimSpace(line), " ")
		if !ok {
			continue
		}
		v, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", key, err)
		}
		values[key] = v
	}
	return values, nil
}
