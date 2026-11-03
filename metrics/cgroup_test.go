package metrics

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeCgroup creates a directory with the given cgroup files.
func fakeCgroup(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

var baseFiles = map[string]string{
	"memory.current": "52428800\n",
	"memory.peak":    "60000000\n",
	"memory.events":  "low 0\nhigh 0\nmax 257\noom 1\noom_kill 1\noom_group_kill 0\n",
	"cpu.stat": "usage_usec 1537551\nuser_usec 1500000\nsystem_usec 37551\n" +
		"nr_periods 30\nnr_throttled 30\nthrottled_usec 1473725\n",
}

func TestReadCgroup(t *testing.T) {
	files := map[string]string{"memory.swap.current": "4096\n", "memory.swap.peak": "106717184\n"}
	for k, v := range baseFiles {
		files[k] = v
	}
	c, err := ReadCgroup(fakeCgroup(t, files))
	if err != nil {
		t.Fatal(err)
	}
	want := Cgroup{
		MemoryBytes:      52428800,
		MemoryPeakBytes:  60000000,
		SwapAvailable:    true,
		SwapBytes:        4096,
		SwapPeakBytes:    106717184,
		OOMKills:         1,
		CPUUsage:         1537551 * time.Microsecond,
		Periods:          30,
		ThrottledPeriods: 30,
		Throttled:        1473725 * time.Microsecond,
	}
	if c != want {
		t.Errorf("ReadCgroup = %+v, want %+v", c, want)
	}
}

func TestReadCgroupWithoutSwap(t *testing.T) {
	c, err := ReadCgroup(fakeCgroup(t, baseFiles))
	if err != nil {
		t.Fatal(err)
	}
	if c.SwapAvailable || c.SwapBytes != 0 {
		t.Errorf("SwapAvailable %v, SwapBytes %d: want false, 0", c.SwapAvailable, c.SwapBytes)
	}
}

func TestReadCgroupMissingMemory(t *testing.T) {
	if _, err := ReadCgroup(t.TempDir()); err == nil {
		t.Error("ReadCgroup of an empty directory returned no error")
	}
}

func TestParseKeyedBadValue(t *testing.T) {
	if _, err := parseKeyed("usage_usec abc\n"); err == nil {
		t.Error("parseKeyed with a bad value returned no error")
	}
}

func TestCgroupOfSelf(t *testing.T) {
	dir, err := CgroupOf(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(dir, CgroupRoot) {
		t.Errorf("CgroupOf = %q, want it under %s", dir, CgroupRoot)
	}
}
