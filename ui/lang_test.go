package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"

	"concurrency-memory-lab/cgroup"
	"concurrency-memory-lab/runner"
	"concurrency-memory-lab/workload"
)

func TestParseLang(t *testing.T) {
	for s, want := range map[string]Lang{"en": English, "ES": Spanish, "español": Spanish, "english": English} {
		if got, err := ParseLang(s); err != nil || got != want {
			t.Errorf("ParseLang(%q) = %v, %v; want %v", s, got, err, want)
		}
	}
	if _, err := ParseLang("fr"); err == nil {
		t.Error("ParseLang(\"fr\") returned no error")
	}
}

func TestLangFromEnv(t *testing.T) {
	tests := []struct {
		all, messages, lang string
		want                Lang
	}{
		{"", "", "es_VE.UTF-8", Spanish},
		{"", "", "C.UTF-8", English},
		{"", "", "", English},
		{"en_US.UTF-8", "", "es_VE.UTF-8", English}, // LC_ALL wins
		{"", "es_ES.UTF-8", "en_US.UTF-8", Spanish}, // then LC_MESSAGES
	}
	for _, tt := range tests {
		t.Setenv("LC_ALL", tt.all)
		t.Setenv("LC_MESSAGES", tt.messages)
		t.Setenv("LANG", tt.lang)
		if got := LangFromEnv(); got != tt.want {
			t.Errorf("LC_ALL=%q LC_MESSAGES=%q LANG=%q: %v, want %v", tt.all, tt.messages, tt.lang, got, tt.want)
		}
	}
}

// TestTextsFit checks, in both languages, that every text fits where it is
// drawn on an 80x24 screen, with large but possible values.
func TestTextsFit(t *testing.T) {
	limits := cgroup.Limits{CPUPercent: 1600, MemoryBytes: 65536 << 20, SwapBytes: 65536 << 20}
	for _, l := range []Lang{English, Spanish} {
		tx := texts(l)
		a := &App{t: tx, lang: l}
		check := func(what, s string, width int) {
			t.Helper()
			if n := utf8.RuneCountInString(s); n > width {
				t.Errorf("lang %d, %s: %d cells, room for %d: %q", l, what, n, width, s)
			}
		}

		check("title", tx.title, 80)
		check("keys", tx.keysMain, 80)
		check("compare keys", tx.keysCompare, 80)
		check("limits ok", tx.limitsOK, 79)
		check("limits not", fmt.Sprintf(tx.limitsNotF, tx.notRoot), 79)
		check("switch lang", tx.switchLang, rightW)
		check("configuration", tx.configuration, leftW)

		for i := range tx.labels {
			check("label", tx.labels[i], 14)
			check("help", tx.helps[i], 79-leftX)
		}
		for _, v := range append(tx.syncLabels[:], tx.none, tx.noSwap) {
			check("field value", v, 20)
		}

		check("live", tx.live, rightW)
		check("idle", tx.idle, rightW)
		check("safe", fmt.Sprintf(tx.safeF, 9999, 9999), rightW)
		check("running", fmt.Sprintf(tx.runningF, 999.9, 4194304), rightW)
		for _, r := range []string{
			row(tx.virtual, "99999.9 MiB"), row(tx.resident, "99999.9 MiB"), row(tx.swap, "9999.9 MiB"),
			row(tx.faults, fmt.Sprintf(tx.faultsF, 999999, 99999)),
			row(tx.threads, fmt.Sprintf(tx.threadsF, 1024, 999*time.Second)),
			// 36000 periods of 100 ms: the longest time limit, one hour.
			row(tx.cgroupMemory, "99999.9 MiB"), row(tx.throttled, fmt.Sprintf(tx.throttledF, 36000, 36000)),
			row(tx.cgroup, tx.none),
		} {
			check("live row", r, rightW)
		}
		for _, name := range []string{"cpu", "memory", "swap"} {
			check("limit", a.limitText(cgroup.LimitResult{Name: name}, limits), rightW)
		}

		for o, s := range tx.outcomes {
			check("outcome "+string(o), s+"  (99.999s)", 79)
		}
		check("work", fmt.Sprintf(tx.workF, int64(999_999_000_000), 999*time.Second), 79)
		check("counter", fmt.Sprintf(tx.counterF, int64(1_999_999), int64(40_000_000), int64(38_000_001), 95.0, 9999*time.Millisecond), 79)
		check("peak", fmt.Sprintf(tx.peakF, "9999.9 MiB", "9999.9 MiB", 999999, 99999), 79)
		cpuLine := fmt.Sprintf(tx.cpuF, 999.99, 99.99) + fmt.Sprintf(tx.cgroupF, 9999, 9999, 1)
		check("cpu line", cpuLine+tx.limitsNotAll, 79)
		check("cpu line", cpuLine+tx.limitsNone, 79)
		check("last result", fmt.Sprintf(tx.lastResultF, 99, a.describe(runner.Config{
			Threads: 64, Mode: workload.OSThreads, MemoryMiB: 2048, WorkMillions: 99999, Sync: workload.SyncAtomic})), 79)

		check("header", tx.tableHeader, 78)
		if n := utf8.RuneCountInString(tx.tableHeader); n != len(compareRow(1, fakeResult(1, time.Second), tx)) {
			t.Errorf("lang %d: header and rows have different widths", l)
		}
		for o, s := range tx.outcomesShort {
			check("short outcome "+string(o), s+"*", 8)
		}
		check("legend", tx.legend, 79)
		check("no results", tx.noResults, 79)

		for _, m := range []string{
			tx.alreadyRunning, tx.started, tx.noFinished, tx.notRunning, tx.stopping, tx.stoppingQuit,
			tx.defaults, tx.locked, tx.typeHint, tx.editCancelled, tx.langChanged,
			fmt.Sprintf(tx.finishedF, tx.outcomes[runner.OutcomeTimeout]),
			fmt.Sprintf(tx.orphansF, 99),
			fmt.Sprintf(tx.setToF, tx.labels[7], tx.noSwap),
			fmt.Sprintf(tx.betweenF, tx.labels[0], 1, 1024),
			tx.unsafeText(&runner.UnsafeError{MiB: 65536, MaxMiB: 65536}),
			tx.unsafeText(&runner.UnsafeError{Swap: true, MiB: 65536, MaxMiB: 65536}),
		} {
			check("message", m, 79)
		}
	}
}

// Every language must have a text for every outcome.
func TestEveryOutcomeIsTranslated(t *testing.T) {
	outcomes := []runner.Outcome{runner.OutcomeCompleted, runner.OutcomeStopped, runner.OutcomeTimeout,
		runner.OutcomeOOM, runner.OutcomeKilled, runner.OutcomeFailed}
	for _, l := range []Lang{English, Spanish} {
		for _, o := range outcomes {
			if texts(l).outcomes[o] == "" || texts(l).outcomesShort[o] == "" {
				t.Errorf("lang %d has no text for %q", l, o)
			}
		}
	}
}

func TestLKeySwitchesTheLanguage(t *testing.T) {
	a, screen := newTestApp(t, 80, 24)
	a.sel = 3
	a.cfg.Threads = 8
	typeText(a, "l")
	a.draw()
	text := screenText(screen)
	for _, want := range []string{"CONFIGURACIÓN", "EN VIVO", "Hilos", "Trabajo de CPU", "idioma: español", "l: switch to English"} {
		if !strings.Contains(text, want) {
			t.Errorf("Spanish screen does not show %q", want)
		}
	}
	if a.sel != 3 || a.cfg.Threads != 8 {
		t.Errorf("the selection or the configuration changed: sel %d, threads %d", a.sel, a.cfg.Threads)
	}

	a.openCompare()
	typeText(a, "l") // also in the compare view
	a.draw()
	if a.lang != English || !strings.Contains(screenText(screen), "no test has finished yet") {
		t.Error("l in the compare view did not switch back to English")
	}
}

func TestSpanishResults(t *testing.T) {
	a, screen := newTestApp(t, 80, 24)
	a.setLang(Spanish)
	cfg := defaultConfig()
	cfg.Sync = workload.SyncNone
	cfg.Limits.MemoryBytes = 128 << 20
	a.results = append(a.results, runner.Result{
		Config:  cfg,
		Outcome: runner.OutcomeOOM,
		Limits:  []cgroup.LimitResult{{Name: "memory", Applied: false}},
		Work:    runner.Work{Reported: true, Shared: true, Done: 4000, Counter: 3000, Elapsed: time.Millisecond},
	})
	a.draw()
	text := screenText(screen)
	for _, want := range []string{
		"ÚLTIMO RESULTADO  prueba 1:", "contador none", "terminada por el kernel",
		"contador: 3000 de 4000 incrementos, perdidos 1000 (25.0%)", "límites: NO aplicados",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("Spanish result does not show %q", want)
		}
	}

	typeText(a, "c")
	a.draw()
	text = screenText(screen)
	for _, want := range []string{"hilos", "perd.", "OOM*", "25.0%", "PRUEBA 1:", "límites: memoria 128 MiB: NO aplicado"} {
		if !strings.Contains(text, want) {
			t.Errorf("Spanish compare view does not show %q", want)
		}
	}
}

func TestSpanishMessages(t *testing.T) {
	a, screen := newTestApp(t, 80, 24)
	a.setLang(Spanish)
	a.limitsErr = errNotRoot
	a.start = func(runner.Config) (*runner.Run, error) {
		return nil, &runner.UnsafeError{MiB: 5000, MaxMiB: 3120}
	}
	typeText(a, "s")
	a.draw()
	text := screenText(screen)
	for _, want := range []string{
		"límites NO disponibles: no es root, ábrelo con sudo",
		"inseguro: 5000 MiB en RAM, el máximo es 3120 MiB (mitad de la RAM disponible)",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("Spanish screen does not show %q", want)
		}
	}

	selectField(t, a, "Hilos")
	key(a, tcell.KeyEnter)
	typeText(a, "5000")
	key(a, tcell.KeyEnter)
	if !strings.Contains(a.message, "Hilos debe estar entre 1 y 1024") {
		t.Errorf("out of range message %q", a.message)
	}
}
