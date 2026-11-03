package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"

	"resource-lab/cgroup"
	"resource-lab/runner"
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

// Layout: the form on the left, the live panel on the right, the help of
// the selected field and the last result under both. Everything fits in
// 80x24: the live panel ends at line 14 and the result at line 21.
const (
	leftX   = 1
	leftW   = 37
	rightX  = 41
	rightW  = 38
	formY   = 3
	helpY   = 16
	resultY = 17
)

func (a *App) draw() {
	a.screen.Clear()
	width, height := a.screen.Size()

	a.put(0, 0, width, styleBar, " resource-lab  -  concurrency and memory lab")
	if a.limitsErr == nil {
		a.put(1, 1, width-1, styleOK, "limits: available (root, cgroup v2)")
	} else {
		a.put(1, 1, width-1, styleBad, "limits NOT available: "+a.limitsErr.Error()+" - tests run without limits")
	}

	a.drawForm()
	a.drawLive()
	a.drawResult()

	if a.message != "" {
		style := styleWarn
		if a.msgIsErr {
			style = styleBad
		}
		a.put(1, height-2, width-1, style, a.message)
	}
	a.put(0, height-1, width, styleBar,
		" arrows move/change  Enter type  s start  x stop  r repeat  d defaults  q quit")
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

// drawResult shows the last finished test.
func (a *App) drawResult() {
	_, height := a.screen.Size()
	y := resultY
	if len(a.results) == 0 {
		return
	}
	res := a.results[len(a.results)-1]
	a.put(leftX, y, 79, styleTitle, fmt.Sprintf("LAST RESULT  test %d: %s", len(a.results), describe(res.Config)))
	y++
	outcomeStyle := styleOK
	if res.Outcome != runner.OutcomeCompleted {
		outcomeStyle = styleWarn
	}
	a.put(leftX, y, 79, outcomeStyle, fmt.Sprintf("%s  (%v)", res.Outcome, res.Duration.Round(time.Millisecond)))
	y++
	swapPeak := res.PeakSwapBytes
	if res.HasCgroup && res.Cgroup.SwapAvailable {
		swapPeak = res.Cgroup.SwapPeakBytes
	}
	lines := []string{
		fmt.Sprintf("peak RAM %s  peak swap %s  faults %d minor %d major",
			mib(res.PeakResidentBytes), a.swapText(swapPeak), res.MinorFaults, res.MajorFaults),
		fmt.Sprintf("cpu time %v user %v system  %s", res.UserTime.Round(time.Millisecond),
			res.SystemTime.Round(time.Millisecond), cpuWorkLine(res.Output)),
	}
	for _, l := range lines {
		if y >= height-2 {
			return
		}
		a.put(leftX, y, 79, styleNormal, l)
		y++
	}
	if y >= height-2 {
		return
	}
	cgroupText := "no cgroup  "
	if res.HasCgroup {
		c := res.Cgroup
		cgroupText = fmt.Sprintf("throttled %d of %d periods (%v)  oom kills %d  ",
			c.ThrottledPeriods, c.Periods, c.Throttled.Round(time.Millisecond), c.OOMKills)
	}
	limitsText, style := limitsSummary(res.Limits)
	x := leftX + len(cgroupText)
	a.put(leftX, y, len(cgroupText), styleNormal, cgroupText)
	a.put(x, y, 80-x, style, limitsText)
}

// limitsSummary says in a few words whether the test was really limited.
func limitsSummary(limits []cgroup.LimitResult) (string, tcell.Style) {
	if len(limits) == 0 {
		return "limits: none requested", styleNormal
	}
	for _, l := range limits {
		if !l.Applied {
			return "limits: NOT applied, the test was not limited", styleBad
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
	return fmt.Sprintf("%d %s, %d MiB, %d M", c.Threads, c.Mode, c.MemoryMiB, c.WorkMillions)
}

// cpuWorkLine returns the worker's "cpu work: ..." line, if it printed one.
func cpuWorkLine(output string) string {
	for line := range strings.Lines(output) {
		if rest, ok := strings.CutPrefix(line, "cpu work: "); ok {
			if i := strings.Index(rest, " (checksum"); i >= 0 {
				rest = rest[:i]
			}
			return "work: " + strings.TrimSpace(rest)
		}
	}
	return ""
}

func mib(b uint64) string {
	return fmt.Sprintf("%.1f MiB", float64(b)/(1<<20))
}
