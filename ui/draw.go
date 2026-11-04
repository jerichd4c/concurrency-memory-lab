package ui

import (
	"errors"
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"

	"concurrency-memory-lab/cgroup"
	"concurrency-memory-lab/runner"
	"concurrency-memory-lab/workload"
)

var (
	styleNormal = tcell.StyleDefault
	styleBar    = tcell.StyleDefault.Reverse(true)
	styleTitle  = tcell.StyleDefault.Bold(true).Underline(true)
	styleDim    = tcell.StyleDefault.Dim(true)
	styleOK     = tcell.StyleDefault.Foreground(tcell.ColorGreen)
	styleBad    = tcell.StyleDefault.Foreground(tcell.ColorRed).Bold(true)
	styleWarn   = tcell.StyleDefault.Foreground(tcell.ColorYellow)
)

// Layout of the main view: the form on the left, the live panel on the
// right, the help of the selected field and the last result under both.
// Everything fits in 80x24: the live panel ends at line 14 and the result
// at line 21. The compare view is laid out in compare.go.
const (
	leftX   = 1
	leftW   = 37
	rightX  = 41
	rightW  = 38
	formY   = 3
	helpY   = 16
	resultY = 17
)

// minWidth and minHeight are the smallest terminal the layout fits in.
const (
	minWidth  = 80
	minHeight = 24
)

func (a *App) draw() {
	a.screen.Clear()
	width, height := a.screen.Size()
	t := a.t
	if width < minWidth || height < minHeight {
		// Drawing the screens cut would hide values and mix columns.
		a.put(0, 0, width, styleBad, t.tooSmall)
		a.put(0, 1, width, styleNormal, fmt.Sprintf(t.tooSmallF, width, height, minWidth, minHeight))
		a.put(0, 2, width, styleDim, t.quitHint)
		a.screen.Show()
		return
	}

	a.put(0, 0, width, styleBar, t.title)
	if a.limitsErr == nil {
		a.put(1, 1, width-1, styleOK, t.limitsOK)
	} else {
		reason := a.limitsErr.Error()
		if errors.Is(a.limitsErr, errNotRoot) {
			reason = t.notRoot
		}
		a.put(1, 1, width-1, styleBad, fmt.Sprintf(t.limitsNotF, reason))
	}
	a.put(width-1-len([]rune(t.switchLang)), 2, len([]rune(t.switchLang)), styleDim, t.switchLang)

	keys := t.keysMain
	if a.view == viewCompare {
		a.drawCompare()
		keys = t.keysCompare
	} else {
		a.drawForm()
		a.drawLive()
		if n := len(a.results); n > 0 {
			a.drawResult(resultY, fmt.Sprintf(t.lastResultF, n, a.describe(a.results[n-1].Config)), a.results[n-1])
		}
	}

	if a.message != "" {
		style := styleWarn
		if a.msgIsErr {
			style = styleBad
		}
		a.put(1, height-2, width-1, style, a.message)
	}
	a.put(0, height-1, width, styleBar, keys)
	a.screen.Show()
}

func (a *App) drawForm() {
	a.put(leftX, formY, leftW, styleTitle, a.t.configuration)
	for i, f := range a.fields {
		style := styleNormal
		if a.run != nil {
			style = styleDim // locked while a test runs
		}
		value := f.format(f.get(&a.cfg))
		if i == a.sel {
			style = style.Reverse(true)
			if a.editing {
				value = a.edit + "_"
			}
		}
		a.put(leftX, formY+1+i, leftW, style, fmt.Sprintf(" %-14s %20s ", f.label, value))
	}
	a.put(leftX, helpY, 79-leftX, styleDim, a.fields[a.sel].help)
}

func (a *App) drawLive() {
	t := a.t
	y := formY
	a.put(rightX, y, rightW, styleTitle, t.live)
	y++
	if a.run == nil {
		a.put(rightX, y, rightW, styleDim, t.idle)
		if a.system.MemTotalBytes != 0 {
			ram, swap := runner.SafeMemory(a.system)
			a.put(rightX, y+1, rightW, styleDim, fmt.Sprintf(t.safeF, ram>>20, swap>>20))
		}
		return
	}
	a.put(rightX, y, rightW, styleWarn, fmt.Sprintf(t.runningF, a.sample.Elapsed.Seconds(), a.run.Pid()))
	y++
	if !a.hasSample {
		return
	}
	p := a.sample.Process
	lines := []string{
		row(t.virtual, mib(p.VirtualBytes)),
		row(t.resident, mib(p.ResidentBytes)),
		row(t.swap, a.swapText(p.SwapBytes)),
		row(t.faults, fmt.Sprintf(t.faultsF, p.MinorFaults, p.MajorFaults)),
		row(t.threads, fmt.Sprintf(t.threadsF, p.Threads, p.CPUTime.Round(10*time.Millisecond))),
	}
	if a.sample.HasCgroup {
		c := a.sample.Cgroup
		lines = append(lines,
			row(t.cgroupMemory, mib(c.MemoryBytes)),
			row(t.throttled, fmt.Sprintf(t.throttledF, c.ThrottledPeriods, c.Periods)))
	} else {
		lines = append(lines, row(t.cgroup, t.none))
	}
	for _, l := range lines {
		a.put(rightX, y, rightW, styleNormal, l)
		y++
	}
	cfg := a.run.Config()
	for _, l := range a.run.Limits() {
		a.put(rightX, y, rightW, limitStyle(l.Applied), a.limitText(l, cfg.Limits))
		y++
	}
}

// drawResult shows one finished test in five lines from line y: the title,
// how it ended, its CPU phase, memory and CPU use. It stops before the
// message line.
func (a *App) drawResult(y int, title string, res runner.Result) {
	t := a.t
	_, height := a.screen.Size()
	if y >= height-2 {
		return
	}
	a.put(leftX, y, 79, styleTitle, title)
	y++
	if y >= height-2 {
		return
	}
	outcomeStyle := styleOK
	if res.Outcome != runner.OutcomeCompleted {
		outcomeStyle = styleWarn
	}
	a.put(leftX, y, 79, outcomeStyle, fmt.Sprintf("%s  (%v)", t.outcomes[res.Outcome], res.Duration.Round(time.Millisecond)))
	y++
	if y >= height-2 {
		return
	}
	work, workStyle := a.workText(res.Work)
	a.put(leftX, y, 79, workStyle, work)
	y++

	if y >= height-2 {
		return
	}
	a.put(leftX, y, 79, styleNormal, fmt.Sprintf(t.peakF,
		mib(res.PeakResidentBytes), a.swapText(peakSwap(res)), res.MinorFaults, res.MajorFaults))
	y++

	if y >= height-2 {
		return
	}
	text := fmt.Sprintf(t.cpuF, res.UserTime.Seconds(), res.SystemTime.Seconds())
	if res.HasCgroup {
		c := res.Cgroup
		text += fmt.Sprintf(t.cgroupF, c.ThrottledPeriods, c.Periods, c.OOMKills)
	} else {
		text += t.noCgroup
	}
	limitsText, style := a.limitsSummary(res.Limits)
	n := len([]rune(text))
	a.put(leftX, y, n, styleNormal, text)
	a.put(leftX+n, y, 79-n, style, limitsText)
}

// peakSwap is the highest swap use of a test: the cgroup counts it exactly,
// sampling only sees it every half second.
func peakSwap(res runner.Result) uint64 {
	if res.HasCgroup && res.Cgroup.SwapAvailable {
		return res.Cgroup.SwapPeakBytes
	}
	return res.PeakSwapBytes
}

// workText describes the CPU phase of a result. A shared counter that lost
// updates is shown in red: that is the race condition.
func (a *App) workText(w runner.Work) (string, tcell.Style) {
	switch {
	case !w.Reported:
		return a.t.workUnfinished, styleDim
	case !w.Shared:
		return fmt.Sprintf(a.t.workF, w.Done, w.Elapsed), styleNormal
	}
	lost := w.Lost()
	text := fmt.Sprintf(a.t.counterF, w.Counter, w.Done, lost, percent(lost, w.Done), w.Elapsed)
	if lost > 0 {
		return text, styleBad
	}
	return text, styleOK
}

func percent(part, whole int64) float64 {
	if whole == 0 {
		return 0
	}
	return 100 * float64(part) / float64(whole)
}

// limitsSummary says in a few words whether the test was really limited.
func (a *App) limitsSummary(limits []cgroup.LimitResult) (string, tcell.Style) {
	if len(limits) == 0 {
		return a.t.limitsNone, styleNormal
	}
	for _, l := range limits {
		if !l.Applied {
			return a.t.limitsNotAll, styleBad
		}
	}
	return a.t.limitsAll, styleOK
}

// swapText shows swap, or says the machine has none.
func (a *App) swapText(b uint64) string {
	if a.system.MemTotalBytes != 0 && a.system.SwapTotalBytes == 0 {
		return a.t.noSwapMachine
	}
	return mib(b)
}

// put writes text at (x, y), cut or padded to width cells. It counts runes,
// not bytes, so letters like á or ñ take one cell.
func (a *App) put(x, y, width int, style tcell.Style, text string) {
	sw, sh := a.screen.Size()
	if y < 0 || y >= sh {
		return
	}
	for _, r := range text {
		if width <= 0 || x >= sw {
			return
		}
		a.screen.SetContent(x, y, r, nil, style)
		x++
		width--
	}
	for ; width > 0 && x < sw; width-- {
		a.screen.SetContent(x, y, ' ', nil, style)
		x++
	}
}

// row is one line of the live panel: a label and its value.
func row(label, value string) string {
	return fmt.Sprintf("%-15s %s", label, value)
}

// limitText says whether one limit was applied, for example
// "memory 128 MiB: applied". The requested value is written again from the
// configuration, in the language of the screen.
func (a *App) limitText(l cgroup.LimitResult, want cgroup.Limits) string {
	var value string
	switch l.Name {
	case "cpu":
		value = fmt.Sprintf(a.t.cpuLimitF, want.CPUPercent)
	case "memory":
		value = fmt.Sprintf("%d MiB", want.MemoryBytes>>20)
	case "swap":
		value = fmt.Sprintf("%d MiB", want.SwapBytes>>20)
	}
	status := a.t.applied
	if !l.Applied {
		status = a.t.notApplied
	}
	return a.t.limitNames[l.Name] + " " + value + ": " + status
}

func limitStyle(applied bool) tcell.Style {
	if applied {
		return styleOK
	}
	return styleBad
}

// describe summarizes a configuration in one line.
func (a *App) describe(c runner.Config) string {
	s := fmt.Sprintf("%d %s, %d MiB, %d M", c.Threads, c.Mode, c.MemoryMiB, c.WorkMillions)
	if c.Sync != "" && c.Sync != workload.SyncOff {
		s += ", " + a.t.counterWord + " " + string(c.Sync)
	}
	return s
}

func mib(b uint64) string {
	return fmt.Sprintf("%.1f MiB", float64(b)/(1<<20))
}
