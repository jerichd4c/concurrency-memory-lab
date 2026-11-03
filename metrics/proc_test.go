package metrics

import (
	"os"
	"testing"
	"time"
)

const sampleStatus = `Name:	concurrency-mem
State:	S (sleeping)
Pid:	2644
VmPeak:	 1489740 kB
VmSize:	 1489740 kB
VmHWM:	  264920 kB
VmRSS:	  264920 kB
VmSwap:	    1024 kB
Threads:	8
`

func TestParseStatus(t *testing.T) {
	var p Process
	if err := parseStatus(sampleStatus, &p); err != nil {
		t.Fatal(err)
	}
	want := Process{
		VirtualBytes:      1489740 * 1024,
		ResidentBytes:     264920 * 1024,
		PeakResidentBytes: 264920 * 1024,
		SwapBytes:         1024 * 1024,
		Threads:           8,
	}
	if p != want {
		t.Errorf("parseStatus = %+v, want %+v", p, want)
	}
}

func TestParseStat(t *testing.T) {
	// comm contains spaces and ')' on purpose. Fields 10, 12, 14 and 15
	// (minflt, majflt, utime, stime) are 131470, 41, 250 and 50.
	const stat = "2644 (my (odd) name) S 1 2644 2644 0 -1 4194304 131470 0 41 0 250 50 0 0 20 0 8 0 100 1525493760 66230 18446744073709551615"
	var p Process
	if err := parseStat(stat, &p); err != nil {
		t.Fatal(err)
	}
	if p.MinorFaults != 131470 || p.MajorFaults != 41 {
		t.Errorf("faults = %d minor, %d major; want 131470, 41", p.MinorFaults, p.MajorFaults)
	}
	if p.CPUTime != 3*time.Second {
		t.Errorf("CPUTime = %v, want 3s", p.CPUTime)
	}
}

func TestParseStatErrors(t *testing.T) {
	for _, stat := range []string{"2644 no parentheses", "2644 (x) S 1 2", "2644 (x) S 1 2 3 4 5 6 abc 0 0 0 0 0"} {
		var p Process
		if err := parseStat(stat, &p); err == nil {
			t.Errorf("parseStat(%q) returned no error", stat)
		}
	}
}

func TestReadProcessSelf(t *testing.T) {
	p, err := ReadProcess(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	if p.ResidentBytes == 0 || p.VirtualBytes < p.ResidentBytes {
		t.Errorf("virtual %d, resident %d: want 0 < resident <= virtual", p.VirtualBytes, p.ResidentBytes)
	}
	if p.Threads < 1 || p.MinorFaults == 0 {
		t.Errorf("threads %d, minor faults %d: want both > 0", p.Threads, p.MinorFaults)
	}
}

func TestReadProcessMissing(t *testing.T) {
	if _, err := ReadProcess(-1); err == nil {
		t.Error("ReadProcess(-1) returned no error")
	}
}
