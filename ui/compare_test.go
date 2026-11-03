package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"

	"concurrency-memory-lab/cgroup"
	"concurrency-memory-lab/runner"
	"concurrency-memory-lab/workload"
)

// fakeResult is a finished test with the given threads and duration.
func fakeResult(threads int, d time.Duration) runner.Result {
	cfg := defaultConfig()
	cfg.Threads = threads
	return runner.Result{
		Config:            cfg,
		Outcome:           runner.OutcomeCompleted,
		Duration:          d,
		PeakResidentBytes: 259 << 20,
		Work:              runner.Work{Reported: true, Done: 2_000_000_000, Elapsed: d},
	}
}

func TestCompareRowLinesUpWithTheHeader(t *testing.T) {
	res := fakeResult(16, 13400*time.Millisecond)
	res.Config.Mode = workload.OSThreads
	res.Config.Sync = workload.SyncMutex
	res.Config.Limits = cgroup.Limits{CPUPercent: 50, MemoryBytes: 128 << 20, SwapBytes: 0}
	res.Limits = []cgroup.LimitResult{{Name: "cpu", Applied: true}}
	res.MajorFaults = 123456
	res.Work = runner.Work{Reported: true, Shared: true, Done: 1000, Counter: 1000}

	row := compareRow(12, res)
	want := fmt.Sprintf("%2d %2d %-3s %-6s %4d %5d %-11s %-8s %6s %4d %4d %5s %5s",
		12, 16, "os", "mutex", 256, 2000, "c50 m128 s0", "ok", "13.4s", 259, 0, "123k", "0.0%")
	if row != want {
		t.Errorf("row\n got %q\nwant %q", row, want)
	}
	if len(row) != len(tableHeader) {
		t.Errorf("row is %d cells, header %d: columns do not line up", len(row), len(tableHeader))
	}
	if len(tableHeader) > 78 {
		t.Errorf("header is %d cells, does not fit in 80 columns", len(tableHeader))
	}
}

func TestLimitsShort(t *testing.T) {
	tests := []struct {
		l    cgroup.Limits
		want string
	}{
		{cgroup.Limits{SwapBytes: -1}, "-"},
		{cgroup.Limits{CPUPercent: 200, SwapBytes: -1}, "c200"},
		{cgroup.Limits{MemoryBytes: 128 << 20, SwapBytes: 0}, "m128 s0"},
		{cgroup.Limits{CPUPercent: 25, MemoryBytes: 64 << 20, SwapBytes: 64 << 20}, "c25 m64 s64"},
	}
	for _, tt := range tests {
		if got := limitsShort(tt.l); got != tt.want {
			t.Errorf("limitsShort(%+v) = %q, want %q", tt.l, got, tt.want)
		}
	}
}

func TestOutcomeShortMarksUnlimitedTests(t *testing.T) {
	res := runner.Result{Outcome: runner.OutcomeOOM}
	if got := outcomeShort(res); got != "OOM" {
		t.Errorf("outcomeShort = %q, want OOM", got)
	}
	res = runner.Result{Outcome: runner.OutcomeCompleted, Limits: []cgroup.LimitResult{{Name: "cpu", Applied: false}}}
	if got := outcomeShort(res); got != "ok*" {
		t.Errorf("outcomeShort with a limit not applied = %q, want ok*", got)
	}
}

func TestShortFormats(t *testing.T) {
	durations := map[time.Duration]string{
		1971 * time.Millisecond:  "1.97s",
		13400 * time.Millisecond: "13.4s",
		125 * time.Second:        "125s",
	}
	for d, want := range durations {
		if got := shortDuration(d); got != want {
			t.Errorf("shortDuration(%v) = %q, want %q", d, got, want)
		}
	}
	if shortCount(99999) != "99999" || shortCount(123456) != "123k" {
		t.Errorf("shortCount: %q, %q", shortCount(99999), shortCount(123456))
	}
}

func TestScrollTop(t *testing.T) {
	tests := []struct{ top, sel, rows, want int }{
		{0, 3, 5, 0},   // visible: no change
		{0, 7, 5, 3},   // below: scroll down
		{6, 2, 5, 2},   // above: scroll up
		{3, 7, 5, 3},   // last visible row
		{0, 0, 1, 0},   // a single row
		{0, 20, 1, 20}, // a single row, far away
	}
	for _, tt := range tests {
		if got := scrollTop(tt.top, tt.sel, tt.rows); got != tt.want {
			t.Errorf("scrollTop(%d, %d, %d) = %d, want %d", tt.top, tt.sel, tt.rows, got, tt.want)
		}
	}
}

func TestCompareViewOpensOnTheNewestAndCloses(t *testing.T) {
	a, screen := newTestApp(t, 80, 24)
	typeText(a, "c")
	a.draw()
	if a.view != viewCompare || !strings.Contains(screenText(screen), "no test has finished yet") {
		t.Fatal("c with no results does not show the empty compare view")
	}
	key(a, tcell.KeyEscape)
	if a.view != viewMain {
		t.Fatal("Esc does not go back to the main view")
	}

	for i := 1; i <= 3; i++ {
		a.results = append(a.results, fakeResult(i, time.Duration(i)*time.Second))
	}
	typeText(a, "c")
	if a.cmpSel != 2 {
		t.Errorf("compare opened on row %d, want the newest (2)", a.cmpSel)
	}
	a.draw()
	text := screenText(screen)
	for _, want := range []string{"thread", "majfl", "TEST 3: 3 goroutine", "limits: none requested"} {
		if !strings.Contains(text, want) {
			t.Errorf("compare view does not show %q", want)
		}
	}
	typeText(a, "c")
	if a.view != viewMain {
		t.Error("c does not go back to the main view")
	}
}

func TestCompareSelectionAndScroll(t *testing.T) {
	a, screen := newTestApp(t, 80, 24)
	for i := 1; i <= 30; i++ {
		a.results = append(a.results, fakeResult(i, time.Second))
	}
	a.openCompare()
	key(a, tcell.KeyHome)
	key(a, tcell.KeyUp) // already on top: stays
	if a.cmpSel != 0 {
		t.Errorf("Up on the first row: sel %d, want 0", a.cmpSel)
	}
	key(a, tcell.KeyPgDn)
	key(a, tcell.KeyPgDn)
	key(a, tcell.KeyDown)
	if a.cmpSel != 21 {
		t.Errorf("after PgDn PgDn Down: sel %d, want 21", a.cmpSel)
	}
	a.draw() // the selected row must be scrolled into view
	if !strings.Contains(screenText(screen), "TEST 22: 22 goroutine") {
		t.Error("the detail does not follow the selection")
	}
	if a.cmpTop > 21 || a.cmpTop+11 <= 21 {
		t.Errorf("row 21 not visible: top %d", a.cmpTop)
	}
	key(a, tcell.KeyEnd)
	key(a, tcell.KeyDown) // already on the last row: stays
	if a.cmpSel != 29 {
		t.Errorf("Down on the last row: sel %d, want 29", a.cmpSel)
	}
}

func TestRepeatSelectedLoadsItsConfig(t *testing.T) {
	a, _ := newTestApp(t, 80, 24)
	a.results = append(a.results, fakeResult(2, time.Second), fakeResult(8, time.Second))
	a.openCompare()
	key(a, tcell.KeyUp)
	typeText(a, "r")
	if a.cfg.Threads != 2 {
		t.Errorf("r on test 1 set Threads %d, want 2", a.cfg.Threads)
	}
	// newTestApp's start always fails, so the error must be on screen.
	if !a.msgIsErr || !strings.Contains(a.message, "cannot start") {
		t.Errorf("message %q, error %v", a.message, a.msgIsErr)
	}
}

func TestNewResultIsFollowedWhenTheNewestIsSelected(t *testing.T) {
	a, _ := newTestApp(t, 80, 24)
	a.results = append(a.results, fakeResult(1, time.Second), fakeResult(2, time.Second))
	a.openCompare() // on the newest: row 1
	a.addResult(fakeResult(3, time.Second))
	if a.cmpSel != 2 {
		t.Errorf("sel %d, want 2 (the new test)", a.cmpSel)
	}
	key(a, tcell.KeyHome) // an older test selected: it stays selected
	a.addResult(fakeResult(4, time.Second))
	if a.cmpSel != 0 {
		t.Errorf("sel %d, want 0", a.cmpSel)
	}
}

func TestCompareOnTinyScreens(t *testing.T) {
	a, screen := newTestApp(t, 80, 24)
	for i := 1; i <= 5; i++ {
		a.results = append(a.results, fakeResult(i, time.Second))
	}
	a.openCompare()
	for _, size := range [][2]int{{1, 1}, {20, 5}, {80, 10}, {200, 60}} {
		screen.SetSize(size[0], size[1])
		a.draw() // must not panic
	}
}
