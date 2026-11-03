package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"

	"concurrency-memory-lab/cgroup"
	"concurrency-memory-lab/runner"
	"concurrency-memory-lab/workload"
)

// view is what fills the screen between the title and the key bar.
type view int

const (
	viewMain    view = iota // form, live panel and last result
	viewCompare             // table of every finished test
)

// Layout of the compare view: the table from line 3, a legend, and the
// detail of the selected test in the last lines before the message.
const (
	tableY      = 3
	detailLines = 6 // title, outcome, work, memory, CPU, limits
)

// The table columns. Each row is formatted with rowFormat and the header
// uses the same widths, so the columns line up. It is 77 cells wide.
const (
	headerFormat = "%2s %-6s %-6s %4s %5s %-11s %-8s %6s %4s %4s %5s %5s"
	rowFormat    = "%2d %2d %-3s %-6s %4d %5d %-11s %-8s %6s %4d %4d %5s %5s"
)

var tableHeader = fmt.Sprintf(headerFormat,
	"#", "thread", "sync", "mem", "work", "limits", "result", "time", "RAM", "swap", "majfl", "lost")

func (a *App) openCompare() {
	a.view = viewCompare
	a.editing = false
	a.cmpSel = max(len(a.results)-1, 0)
	a.message = ""
}

// drawCompare draws the table and the detail of the selected test.
func (a *App) drawCompare() {
	_, height := a.screen.Size()
	if len(a.results) == 0 {
		a.put(leftX, tableY, 79, styleDim, "no test has finished yet: run some tests, then compare them here")
		return
	}
	a.put(leftX, tableY, 79, styleTitle, tableHeader)

	detailY := height - 2 - detailLines
	legendY := detailY - 1
	rows := legendY - tableY - 1 // lines left for the table body
	if rows < 1 {
		rows = 1
	}
	a.cmpTop = scrollTop(a.cmpTop, a.cmpSel, rows)
	for i := a.cmpTop; i < len(a.results) && i < a.cmpTop+rows; i++ {
		res := a.results[i]
		style := styleNormal
		if res.Work.Lost() > 0 {
			style = styleBad
		}
		if i == a.cmpSel {
			style = style.Reverse(true)
		}
		a.put(leftX, tableY+1+i-a.cmpTop, 79, style, compareRow(i+1, res))
	}
	a.put(leftX, legendY, 79, styleDim,
		"time: whole test  RAM/swap: peak MiB  * = limits asked but NOT applied")

	res := a.results[a.cmpSel]
	a.drawResult(detailY, fmt.Sprintf("TEST %d: %s", a.cmpSel+1, describe(res.Config)), res)
	// drawResult already says whether the limits were applied; when some
	// were asked for, this line lists each one.
	if y := detailY + detailLines - 1; y < height-2 && len(res.Limits) > 0 {
		a.put(leftX, y, 79, limitsStyle(res.Limits), limitsDetail(res.Limits))
	}
}

// scrollTop returns the first visible row so that sel is on screen.
func scrollTop(top, sel, rows int) int {
	if sel < top {
		return sel
	}
	if sel >= top+rows {
		return sel - rows + 1
	}
	return top
}

// compareRow is one line of the table.
func compareRow(n int, res runner.Result) string {
	c := res.Config
	threadType := "gor"
	if c.Mode == workload.OSThreads {
		threadType = "os"
	}
	syncMode := "-"
	if c.Sync != "" && c.Sync != workload.SyncOff {
		syncMode = string(c.Sync)
	}
	lost := "-"
	if res.Work.Reported && res.Work.Shared {
		lost = fmt.Sprintf("%.1f%%", percent(res.Work.Lost(), res.Work.Done))
	}
	return fmt.Sprintf(rowFormat, n, c.Threads, threadType, syncMode, c.MemoryMiB, c.WorkMillions,
		limitsShort(c.Limits), outcomeShort(res), shortDuration(res.Duration),
		res.PeakResidentBytes>>20, peakSwap(res)>>20, shortCount(res.MajorFaults), lost)
}

// limitsShort writes the requested limits compactly: c50 = CPU 50 %,
// m128 = memory 128 MiB, s0 = swap 0 MiB.
func limitsShort(l cgroup.Limits) string {
	var parts []string
	if l.CPUPercent > 0 {
		parts = append(parts, fmt.Sprintf("c%d", l.CPUPercent))
	}
	if l.MemoryBytes > 0 {
		parts = append(parts, fmt.Sprintf("m%d", l.MemoryBytes>>20))
	}
	if l.SwapBytes >= 0 {
		parts = append(parts, fmt.Sprintf("s%d", l.SwapBytes>>20))
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, " ")
}

// outcomeShort is the outcome in one word, with * when the test asked for
// limits that were not applied: it was not really limited.
func outcomeShort(res runner.Result) string {
	words := map[runner.Outcome]string{
		runner.OutcomeCompleted: "ok",
		runner.OutcomeStopped:   "stopped",
		runner.OutcomeTimeout:   "timeout",
		runner.OutcomeOOM:       "OOM",
		runner.OutcomeKilled:    "killed",
		runner.OutcomeFailed:    "failed",
	}
	word := words[res.Outcome]
	for _, l := range res.Limits {
		if !l.Applied {
			return word + "*"
		}
	}
	return word
}

// shortDuration fits a duration in 6 cells.
func shortDuration(d time.Duration) string {
	s := d.Seconds()
	switch {
	case s < 10:
		return fmt.Sprintf("%.2fs", s)
	case s < 100:
		return fmt.Sprintf("%.1fs", s)
	}
	return fmt.Sprintf("%.0fs", s)
}

// shortCount fits a count in 5 cells.
func shortCount(n uint64) string {
	if n < 100000 {
		return fmt.Sprint(n)
	}
	return fmt.Sprintf("%dk", n/1000)
}

// limitsDetail lists every requested limit and whether it was applied.
func limitsDetail(limits []cgroup.LimitResult) string {
	var parts []string
	for _, l := range limits {
		parts = append(parts, limitText(l.Name, l.Requested, l.Applied))
	}
	return "limits: " + strings.Join(parts, ", ")
}

func limitsStyle(limits []cgroup.LimitResult) tcell.Style {
	_, style := limitsSummary(limits)
	return style
}

// compareKey handles a key in the compare view.
func (a *App) compareKey(ev *tcell.EventKey) {
	last := len(a.results) - 1
	switch ev.Key() {
	case tcell.KeyCtrlC:
		a.quit()
	case tcell.KeyEscape:
		a.view = viewMain
	case tcell.KeyUp:
		a.cmpSel = max(a.cmpSel-1, 0)
	case tcell.KeyDown:
		a.cmpSel = max(min(a.cmpSel+1, last), 0)
	case tcell.KeyPgUp:
		a.cmpSel = max(a.cmpSel-10, 0)
	case tcell.KeyPgDn:
		a.cmpSel = max(min(a.cmpSel+10, last), 0)
	case tcell.KeyHome:
		a.cmpSel = 0
	case tcell.KeyEnd:
		a.cmpSel = max(last, 0)
	case tcell.KeyRune:
		switch ev.Rune() {
		case 'c':
			a.view = viewMain
		case 'q':
			a.quit()
		case 's':
			a.startTest()
		case 'x':
			a.stopTest()
		case 'k':
			a.cmpSel = max(a.cmpSel-1, 0)
		case 'j':
			a.cmpSel = max(min(a.cmpSel+1, last), 0)
		case 'r':
			a.repeatSelected()
		}
	}
}

// repeatSelected runs again the configuration of the selected test; the
// form takes it too, so it can be changed and run once more.
func (a *App) repeatSelected() {
	if len(a.results) == 0 {
		a.setError("no test has finished yet")
		return
	}
	if a.run != nil {
		a.setError("a test is already running")
		return
	}
	a.cfg = a.results[a.cmpSel].Config
	a.startTest()
}
