package metrics

import "testing"

func TestParseMeminfo(t *testing.T) {
	const meminfo = `MemTotal:        7016352 kB
MemFree:         5000000 kB
MemAvailable:    6292348 kB
SwapCached:            0 kB
SwapTotal:       2097152 kB
SwapFree:        2097152 kB
`
	s, err := parseMeminfo(meminfo)
	if err != nil {
		t.Fatal(err)
	}
	want := System{
		MemTotalBytes:     7016352 * 1024,
		MemAvailableBytes: 6292348 * 1024,
		SwapTotalBytes:    2097152 * 1024,
		SwapFreeBytes:     2097152 * 1024,
	}
	if s != want {
		t.Errorf("parseMeminfo = %+v, want %+v", s, want)
	}
}

func TestParseMeminfoWithoutMemTotal(t *testing.T) {
	if _, err := parseMeminfo("SwapTotal: 0 kB\n"); err == nil {
		t.Error("parseMeminfo without MemTotal returned no error")
	}
}

func TestReadSystem(t *testing.T) {
	s, err := ReadSystem()
	if err != nil {
		t.Fatal(err)
	}
	if s.MemAvailableBytes == 0 || s.MemAvailableBytes > s.MemTotalBytes {
		t.Errorf("available %d, total %d: want 0 < available <= total", s.MemAvailableBytes, s.MemTotalBytes)
	}
}
