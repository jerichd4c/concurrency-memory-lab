package workload

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
)

// statusKB reads a "Name:   123 kB" field from /proc/self/status.
func statusKB(t *testing.T, name string) int {
	t.Helper()
	f, err := os.Open("/proc/self/status")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) >= 2 && fields[0] == name+":" {
			n, err := strconv.Atoi(fields[1])
			if err != nil {
				t.Fatal(err)
			}
			return n
		}
	}
	t.Fatalf("%s not found in /proc/self/status", name)
	return 0
}

func TestReserveGrowsVirtualTouchGrowsRSS(t *testing.T) {
	const size = 64 << 20
	const sizeKB = size >> 10

	virt0, rss0 := statusKB(t, "VmSize"), statusKB(t, "VmRSS")
	r, err := Reserve(size)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Release()
	virt1, rss1 := statusKB(t, "VmSize"), statusKB(t, "VmRSS")

	if virt1-virt0 < sizeKB {
		t.Errorf("VmSize grew %d kB after Reserve, want >= %d", virt1-virt0, sizeKB)
	}
	if rss1-rss0 > sizeKB/4 {
		t.Errorf("VmRSS grew %d kB after Reserve, want almost nothing", rss1-rss0)
	}

	var stop atomic.Bool
	if got, want := r.Touch(&stop), size/os.Getpagesize(); got != want {
		t.Errorf("Touch wrote %d pages, want %d", got, want)
	}
	if rss2 := statusKB(t, "VmRSS"); rss2-rss1 < sizeKB*3/4 {
		t.Errorf("VmRSS grew %d kB after Touch, want about %d", rss2-rss1, sizeKB)
	}
}

func TestTouchStops(t *testing.T) {
	r, err := Reserve(1 << 20)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Release()
	var stop atomic.Bool
	stop.Store(true)
	if got := r.Touch(&stop); got != 0 {
		t.Errorf("Touch with stop set wrote %d pages, want 0", got)
	}
}

func TestReleaseTwice(t *testing.T) {
	r, err := Reserve(1 << 20)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Release(); err != nil {
		t.Fatal(err)
	}
	if err := r.Release(); err != nil {
		t.Errorf("second Release: %v", err)
	}
}
