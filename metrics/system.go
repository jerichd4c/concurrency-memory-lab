package metrics

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// System holds machine-wide memory values from /proc/meminfo.
type System struct {
	MemTotalBytes     uint64 // physical RAM
	MemAvailableBytes uint64 // RAM that can be used without swapping
	SwapTotalBytes    uint64 // 0 when the machine has no swap
	SwapFreeBytes     uint64
}

// ReadSystem reads /proc/meminfo.
func ReadSystem() (System, error) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return System{}, err
	}
	return parseMeminfo(string(data))
}

// parseMeminfo reads the "Name:  value kB" lines that System needs.
func parseMeminfo(text string) (System, error) {
	var s System
	fields := map[string]*uint64{
		"MemTotal":     &s.MemTotalBytes,
		"MemAvailable": &s.MemAvailableBytes,
		"SwapTotal":    &s.SwapTotalBytes,
		"SwapFree":     &s.SwapFreeBytes,
	}
	for line := range strings.Lines(text) {
		name, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		if dst, ok := fields[name]; ok {
			bytes, err := parseKB(value)
			if err != nil {
				return System{}, fmt.Errorf("meminfo %s: %w", name, err)
			}
			*dst = bytes
		}
	}
	if s.MemTotalBytes == 0 {
		return System{}, errors.New("meminfo: MemTotal not found")
	}
	return s, nil
}
