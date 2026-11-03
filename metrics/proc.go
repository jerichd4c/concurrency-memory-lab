// Package metrics reads the measurements of the test load: per-process
// values from /proc/<pid>, per-cgroup values from /sys/fs/cgroup and
// machine-wide values from /proc/meminfo.
package metrics

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// clockTicks is USER_HZ, the unit of utime and stime in /proc/<pid>/stat.
// It is 100 on every mainstream Linux architecture.
const clockTicks = 100

// Process holds the memory and CPU measurements of one process.
type Process struct {
	VirtualBytes      uint64        // VmSize: all mapped virtual memory
	ResidentBytes     uint64        // VmRSS: pages currently in RAM
	PeakResidentBytes uint64        // VmHWM: highest VmRSS so far
	SwapBytes         uint64        // VmSwap: pages moved out to swap
	Threads           int           // OS threads of the process
	MinorFaults       uint64        // page faults served without disk I/O
	MajorFaults       uint64        // page faults that had to read from disk or swap
	CPUTime           time.Duration // user + system time
}

// ReadProcess reads /proc/<pid>/status and /proc/<pid>/stat.
func ReadProcess(pid int) (Process, error) {
	dir := "/proc/" + strconv.Itoa(pid)
	status, err := os.ReadFile(dir + "/status")
	if err != nil {
		return Process{}, err
	}
	stat, err := os.ReadFile(dir + "/stat")
	if err != nil {
		return Process{}, err
	}
	var p Process
	if err := parseStatus(string(status), &p); err != nil {
		return Process{}, err
	}
	if err := parseStat(string(stat), &p); err != nil {
		return Process{}, err
	}
	return p, nil
}

// parseStatus fills the memory fields and Threads from the "Name:  value"
// lines of /proc/<pid>/status. Memory values there are in kB.
func parseStatus(text string, p *Process) error {
	memory := map[string]*uint64{
		"VmSize": &p.VirtualBytes,
		"VmRSS":  &p.ResidentBytes,
		"VmHWM":  &p.PeakResidentBytes,
		"VmSwap": &p.SwapBytes,
	}
	for line := range strings.Lines(text) {
		name, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		if name == "Threads" {
			n, err := strconv.Atoi(value)
			if err != nil {
				return fmt.Errorf("status Threads: %w", err)
			}
			p.Threads = n
			continue
		}
		if dst, ok := memory[name]; ok {
			bytes, err := parseKB(value)
			if err != nil {
				return fmt.Errorf("status %s: %w", name, err)
			}
			*dst = bytes
		}
	}
	return nil
}

// parseStat fills the page faults and CPU time from /proc/<pid>/stat.
func parseStat(text string, p *Process) error {
	// Field 2 (comm) is in parentheses and may contain spaces or ')',
	// so the other fields are counted from the last ')'.
	end := strings.LastIndexByte(text, ')')
	if end < 0 {
		return errors.New("stat: missing ')'")
	}
	f := strings.Fields(text[end+1:])
	if len(f) < 13 {
		return errors.New("stat: too few fields")
	}
	// f[0] is field 3 (state), so field n of proc(5) is f[n-3].
	var err error
	field := func(n int) uint64 {
		v, e := strconv.ParseUint(f[n-3], 10, 64)
		if e != nil && err == nil {
			err = fmt.Errorf("stat field %d: %w", n, e)
		}
		return v
	}
	p.MinorFaults = field(10)
	p.MajorFaults = field(12)
	ticks := field(14) + field(15) // utime + stime
	p.CPUTime = time.Duration(ticks) * time.Second / clockTicks
	return err
}

// parseKB converts a value like "2724 kB" into bytes.
func parseKB(value string) (uint64, error) {
	value = strings.TrimSuffix(strings.TrimSpace(value), "kB")
	kb, err := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return 0, err
	}
	return kb * 1024, nil
}
