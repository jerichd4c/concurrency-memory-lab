package ui

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"

	"resource-lab/cgroup"
	"resource-lab/runner"
	"resource-lab/workload"
)

// newTestApp builds an App that draws on an in-memory screen.
func newTestApp(t *testing.T, width, height int) (*App, tcell.SimulationScreen) {
	t.Helper()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(width, height)
	a := newApp(screen, "/nonexistent")
	a.start = func(runner.Config) (*runner.Run, error) { return nil, errors.New("not in tests") }
	return a, screen
}

// screenText returns everything on the screen, one line per row.
func screenText(screen tcell.SimulationScreen) string {
	cells, width, height := screen.GetContents()
	var b strings.Builder
	for y := range height {
		for x := range width {
			b.WriteString(string(cells[y*width+x].Runes))
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func key(a *App, k tcell.Key) {
	a.handleEvent(tcell.NewEventKey(k, 0, tcell.ModNone))
}

func typeText(a *App, s string) {
	for _, r := range s {
		a.handleEvent(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
}

// selectField moves the selection to the field with that label.
func selectField(t *testing.T, a *App, label string) field {
	t.Helper()
	for i, f := range a.fields {
		if f.label == label {
			a.sel = i
			return f
		}
	}
	t.Fatalf("no field %q", label)
	return field{}
}

func TestArrowsChangeTheSelectedField(t *testing.T) {
	a, _ := newTestApp(t, 80, 24)
	key(a, tcell.KeyRight)
	if a.cfg.Threads != 5 {
		t.Errorf("Threads = %d after Right, want 5", a.cfg.Threads)
	}
	key(a, tcell.KeyDown) // Worker type
	key(a, tcell.KeyRight)
	if a.cfg.Mode != "os-thread" {
		t.Errorf("Mode = %q after Right, want os-thread", a.cfg.Mode)
	}
	key(a, tcell.KeyEnter) // Enter moves to the next option, going round
	if a.cfg.Mode != "goroutine" {
		t.Errorf("Mode = %q after Enter, want goroutine", a.cfg.Mode)
	}
}

func TestSharedCounterGoesThroughItsOptions(t *testing.T) {
	a, _ := newTestApp(t, 80, 24)
	f := selectField(t, a, "Shared counter")
	for _, want := range []string{"no sync", "mutex", "atomic", "off"} {
		key(a, tcell.KeyRight)
		if got := f.format(f.get(&a.cfg)); got != want {
			t.Errorf("after Right: %q, want %q", got, want)
		}
	}
	key(a, tcell.KeyLeft)
	if a.cfg.Sync != workload.SyncAtomic {
		t.Errorf("Left from off: Sync = %q, want atomic", a.cfg.Sync)
	}
}

func TestSwapLimitStepsThroughNoneAndZero(t *testing.T) {
	a, _ := newTestApp(t, 80, 24)
	f := selectField(t, a, "Swap limit")
	steps := []struct {
		k    tcell.Key
		want string
	}{
		{tcell.KeyLeft, "none"},
		{tcell.KeyRight, "0 (no swap)"},
		{tcell.KeyRight, "64 MiB"},
		{tcell.KeyLeft, "0 (no swap)"},
		{tcell.KeyLeft, "none"},
	}
	for i, s := range steps {
		key(a, s.k)
		if got := f.format(f.get(&a.cfg)); got != s.want {
			t.Errorf("step %d: swap limit = %q, want %q", i, got, s.want)
		}
	}
	if a.cfg.Limits.SwapBytes != -1 {
		t.Errorf("SwapBytes = %d, want -1 (no limit)", a.cfg.Limits.SwapBytes)
	}
}

func TestTypeAValue(t *testing.T) {
	a, _ := newTestApp(t, 80, 24)
	selectField(t, a, "Memory limit")
	key(a, tcell.KeyEnter)
	typeText(a, "128")
	key(a, tcell.KeyEnter)
	if a.cfg.Limits.MemoryBytes != 128<<20 {
		t.Errorf("MemoryBytes = %d, want %d", a.cfg.Limits.MemoryBytes, 128<<20)
	}

	key(a, tcell.KeyEnter)
	typeText(a, "99999")
	key(a, tcell.KeyEnter)
	if a.cfg.Limits.MemoryBytes != 128<<20 || !a.msgIsErr {
		t.Errorf("out of range value: MemoryBytes %d, error shown %v", a.cfg.Limits.MemoryBytes, a.msgIsErr)
	}

	key(a, tcell.KeyEnter)
	typeText(a, "64")
	key(a, tcell.KeyEscape)
	if a.cfg.Limits.MemoryBytes != 128<<20 || a.editing {
		t.Errorf("Esc did not cancel: MemoryBytes %d, editing %v", a.cfg.Limits.MemoryBytes, a.editing)
	}
}

func TestMinusOnlyWhereNegativeIsAllowed(t *testing.T) {
	a, _ := newTestApp(t, 80, 24)
	selectField(t, a, "Threads")
	key(a, tcell.KeyEnter)
	typeText(a, "-3")
	if a.edit != "3" {
		t.Errorf("typed %q in Threads, want the minus ignored", a.edit)
	}
	key(a, tcell.KeyEscape)

	selectField(t, a, "Swap limit")
	key(a, tcell.KeyEnter)
	typeText(a, "-1")
	key(a, tcell.KeyEnter)
	if a.cfg.Limits.SwapBytes != -1 {
		t.Errorf("SwapBytes = %d after typing -1, want -1", a.cfg.Limits.SwapBytes)
	}
}

func TestFormIsLockedWhileATestRuns(t *testing.T) {
	a, _ := newTestApp(t, 80, 24)
	a.run = &runner.Run{} // only its presence matters here
	key(a, tcell.KeyRight)
	if a.cfg.Threads != 4 || !a.msgIsErr {
		t.Errorf("Threads %d, error shown %v: the form must be locked", a.cfg.Threads, a.msgIsErr)
	}
}

func TestStartErrorIsShown(t *testing.T) {
	a, screen := newTestApp(t, 80, 24)
	typeText(a, "s")
	a.draw()
	if !strings.Contains(screenText(screen), "cannot start: not in tests") {
		t.Error("the start error is not on the screen")
	}
}

func TestRepeatNeedsAFinishedTest(t *testing.T) {
	a, _ := newTestApp(t, 80, 24)
	typeText(a, "r")
	if !a.msgIsErr {
		t.Error("repeat with no finished test showed no error")
	}
}

func TestDrawShowsLimitAvailability(t *testing.T) {
	a, screen := newTestApp(t, 80, 24)
	a.limitsErr = errNotRoot
	a.draw()
	text := screenText(screen)
	for _, want := range []string{"CONFIGURATION", "LIVE", "Threads", "limits NOT available", "idle"} {
		if !strings.Contains(text, want) {
			t.Errorf("screen does not show %q", want)
		}
	}
	a.limitsErr = nil
	a.draw()
	if !strings.Contains(screenText(screen), "limits: available") {
		t.Error("screen does not say limits are available")
	}
}

func TestDrawLastResult(t *testing.T) {
	a, screen := newTestApp(t, 80, 24)
	a.results = append(a.results, runner.Result{
		Config:   defaultConfig(),
		Outcome:  runner.OutcomeOOM,
		Duration: 1500 * time.Millisecond,
		Limits:   []cgroup.LimitResult{{Name: "memory", Requested: "128 MiB", Err: errNotRoot}},
		Work:     runner.Work{Reported: true, Done: 2_000_000_000, Elapsed: 715 * time.Millisecond},
	})
	a.draw()
	text := screenText(screen)
	for _, want := range []string{"LAST RESULT", "out of memory", "1.5s", "work: 2000000000 iterations in 715ms", "limits: NOT applied"} {
		if !strings.Contains(text, want) {
			t.Errorf("screen does not show %q", want)
		}
	}
}

func TestDrawSharedCounterResult(t *testing.T) {
	a, screen := newTestApp(t, 80, 24)
	cfg := defaultConfig()
	cfg.Sync = workload.SyncNone
	a.results = append(a.results, runner.Result{
		Config:  cfg,
		Outcome: runner.OutcomeCompleted,
		Work:    runner.Work{Reported: true, Shared: true, Done: 4000, Counter: 3000, Elapsed: time.Millisecond},
	})
	a.draw()
	text := screenText(screen)
	for _, want := range []string{"counter none", "shared counter: 3000 of 4000 increments, lost 1000 (25.0%)"} {
		if !strings.Contains(text, want) {
			t.Errorf("screen does not show %q", want)
		}
	}
}

func TestDrawOnTinyScreens(t *testing.T) {
	a, screen := newTestApp(t, 80, 24)
	a.results = append(a.results, runner.Result{Config: defaultConfig()})
	for _, size := range [][2]int{{1, 1}, {10, 3}, {40, 10}, {200, 60}} {
		screen.SetSize(size[0], size[1])
		a.draw() // must not panic
	}
}

func TestQuitWhenIdle(t *testing.T) {
	a, _ := newTestApp(t, 80, 24)
	typeText(a, "q")
	if !a.quitting {
		t.Error("q did not quit")
	}
}

func TestWorkText(t *testing.T) {
	if s, _ := workText(runner.Work{}); s != "work: not finished" {
		t.Errorf("workText of an unreported phase = %q", s)
	}
	_, style := workText(runner.Work{Reported: true, Shared: true, Done: 10, Counter: 10})
	if style != styleOK {
		t.Error("an exact shared counter is not shown as OK")
	}
	_, style = workText(runner.Work{Reported: true, Shared: true, Done: 10, Counter: 7})
	if style != styleBad {
		t.Error("a shared counter that lost updates is not shown in red")
	}
}
