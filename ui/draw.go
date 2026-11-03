package ui

import (
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"

	"resource-lab/cgroup"
	"resource-lab/runner"
	"resource-lab/workload"
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
	if width < minWidth || height < minHeight {
		// Drawing the screens cut would hide values and mix columns.
		a.put(0, 0, width, styleBad, "terminal too small")
		a.put(0, 1, width, styleNormal, fmt.Sprintf("%dx%d, needs %dx%d", width, height, minWidth, minHeight))
		a.put(0, 2, width, styleDim, "q quits")
		a.screen.Show()
		return
	}

	a.put(0, 0, width, styleBar, " resource-lab  -  concurrency and memory lab")
	if a.limitsErr == nil {
		a.put(1, 1, width-1, styleOK, "limits: available (root, cgroup v2)")
	} else {
		a.put(1, 1, width-1, styleBad, "limits NOT available: "+a.limitsErr.Error()+" - tests run without limits")
	}

	keys := " arrows move/change  Enter type  s start  x stop  r repeat  c compare  q quit"
	if a.view == viewCompare {
		a.drawCompare()
		keys = " up/down select  r repeat selected  s start  x stop  c/Esc back  q quit"
	} else {
		a.drawForm()
		a.drawLive()
		if n := len(a.results); n > 0 {
			a.drawResult(resultY, fmt.Sprintf("LAST RESULT  test %d: %s", n, describe(a.results[n-1].Config)), a.results[n-1])
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
	a.put(leftX, formY, leftW, styleTitle, "CONFIGURATION")
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
	y := formY
	a.put(rightX, y, rightW, styleTitle, "LIVE")
	y++
	if a.run == nil {
		a.put(rightX, y, rightW, styleDim, "idle - press s to start a test")
		if a.system.MemTotalBytes != 0 {
			ram, swap := runner.SafeMemory(a.system)
			a.put(rightX, y+1, rightW, styleDim, fmt.Sprintf("safe now: %d MiB RAM, %d MiB swap", ram>>20, swap>>20))
		}
		return
	}
	a.put(rightX, y, rightW, styleWarn, fmt.Sprintf("running %.1fs  worker pid %d", a.sample.Elapsed.Seconds(), a.run.Pid()))
	y++
	if !a.hasSample {
		return
	}
	p := a.sample.Process
	lines := []string{
		row("Virtual memory", mib(p.VirtualBytes)),
		row("Resident (RAM)", mib(p.ResidentBytes)),
		row("Swap", a.swapText(p.SwapBytes)),
		row("Page faults", fmt.Sprintf("%d min %d maj", p.MinorFaults, p.MajorFaults)),
		row("Threads", fmt.Sprintf("%-6d CPU time %v", p.Threads, p.CPUTime.Round(10*time.Millisecond))),
	}
	if a.sample.HasCgroup {
		c := a.sample.Cgroup
		lines = append(lines,
			row("Cgroup memory", mib(c.MemoryBytes)),
			row("CPU throttled", fmt.Sprintf("%d of %d periods", c.ThrottledPeriods, c.Periods)))
	} else {
		lines = append(lines, row("Cgroup", "none"))
	}
	for _, l := range lines {
		a.put(rightX, y, rightW, styleNormal, l)
		y++
	}
	for _, l := range a.run.Limits() {
		a.put(rightX, y, rightW, limitStyle(l.Applied), limitText(l.Name, l.Requested, l.Applied))
		y++
	}
}

// drawResult shows one finished test in five lines from line y: the title,
// how it ended, its CPU phase, memory and CPU use. It stops before the
// message line.
func (a *App) drawResult(y int, title string, res runner.Result) {
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
	a.put(leftX, y, 79, outcomeStyle, fmt.Sprintf("%s  (%v)", res.Outcome, res.Duration.Round(time.Millisecond)))
	y++
	if y >= height-2 {
		return
	}
	work, workStyle := workText(res.Work)
	a.put(leftX, y, 79, workStyle, work)
	y++

	if y >= height-2 {
		return
	}
	a.put(leftX, y, 79, styleNormal, fmt.Sprintf("peak RAM %s  peak swap %s  faults %d minor %d major",
		mib(res.PeakResidentBytes), a.swapText(peakSwap(res)), res.MinorFaults, res.MajorFaults))
	y++

	if y >= height-2 {
		return
	}
	text := fmt.Sprintf("cpu %v user %v sys  ", res.UserTime.Round(time.Millisecond), res.SystemTime.Round(time.Millisecond))
	if res.HasCgroup {
		c := res.Cgroup
		text += fmt.Sprintf("throttled %d of %d  oom kills %d  ", c.ThrottledPeriods, c.Periods, c.OOMKills)
	} else {
		text += "no cgroup  "
	}
	limitsText, style := limitsSummary(res.Limits)
	a.put(leftX, y, len(text), styleNormal, text)
	a.put(leftX+len(text), y, 79-len(text), style, limitsText)
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
func workText(w runner.Work) (string, tcell.Style) {
	switch {
	case !w.Reported:
		return "work: not finished", styleDim
	case !w.Shared:
		return fmt.Sprintf("work: %d iterations in %v", w.Done, w.Elapsed), styleNormal
	}
	lost := w.Lost()
	text := fmt.Sprintf("shared counter: %d of %d increments, lost %d (%.1f%%) in %v",
		w.Counter, w.Done, lost, percent(lost, w.Done), w.Elapsed)
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
func limitsSummary(limits []cgroup.LimitResult) (string, tcell.Style) {
	if len(limits) == 0 {
		return "limits: none requested", styleNormal
	}
	for _, l := range limits {
		if !l.Applied {
			return "limits: NOT applied", styleBad
		}
	}
	return "limits: all applied", styleOK
}

// swapText shows swap, or says the machine has none.
func (a *App) swapText(b uint64) string {
	if a.system.MemTotalBytes != 0 && a.system.SwapTotalBytes == 0 {
		return "no swap on this machine"
	}
	return mib(b)
}

// put writes text at (x, y), cut or padded to width cells.
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

func row(label, value string) string {
	return fmt.Sprintf("%-15s %s", label, value)
}

func limitText(name, requested string, applied bool) string {
	if applied {
		return name + " " + requested + ": applied"
	}
	return name + " " + requested + ": NOT applied"
}

func limitStyle(applied bool) tcell.Style {
	if applied {
		return styleOK
	}
	return styleBad
}

// describe summarizes a configuration in one line.
func describe(c runner.Config) string {
	s := fmt.Sprintf("%d %s, %d MiB, %d M", c.Threads, c.Mode, c.MemoryMiB, c.WorkMillions)
	if c.Sync != "" && c.Sync != workload.SyncOff {
		s += ", counter " + string(c.Sync)
	}
	return s
}

func mib(b uint64) string {
	return fmt.Sprintf("%.1f MiB", float64(b)/(1<<20))
}
