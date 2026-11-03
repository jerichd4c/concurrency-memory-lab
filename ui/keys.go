package ui

import (
	"errors"

	"github.com/gdamore/tcell/v2"

	"resource-lab/cgroup"
)

var errNotRoot = errors.New("not root, start it with sudo")

// limitsNone is the configuration with no limit at all.
func limitsNone() cgroup.Limits {
	return cgroup.Limits{SwapBytes: -1}
}

func (a *App) handleEvent(ev tcell.Event) {
	switch ev := ev.(type) {
	case *tcell.EventResize:
		a.screen.Sync()
	case *tcell.EventKey:
		if a.editing {
			a.editKey(ev)
		} else {
			a.commandKey(ev)
		}
	}
}

// commandKey handles a key when the user is not typing a value.
func (a *App) commandKey(ev *tcell.EventKey) {
	switch ev.Key() {
	case tcell.KeyCtrlC:
		a.quit()
		return
	case tcell.KeyUp:
		a.sel = (a.sel + len(a.fields) - 1) % len(a.fields)
		return
	case tcell.KeyDown:
		a.sel = (a.sel + 1) % len(a.fields)
		return
	case tcell.KeyLeft:
		a.change(-1)
		return
	case tcell.KeyRight:
		a.change(+1)
		return
	case tcell.KeyEnter:
		a.beginEdit()
		return
	case tcell.KeyRune:
	default:
		return
	}
	switch ev.Rune() {
	case 'q':
		a.quit()
	case 's':
		a.startTest()
	case 'x':
		a.stopTest()
	case 'r':
		a.repeatTest()
	case 'k':
		a.sel = (a.sel + len(a.fields) - 1) % len(a.fields)
	case 'j':
		a.sel = (a.sel + 1) % len(a.fields)
	case 'd':
		a.cfg = defaultConfig()
		a.setMessage("default configuration")
	}
}

// change moves the selected field. The configuration of a running test
// cannot change, so the form is locked meanwhile.
func (a *App) change(d int64) {
	if a.run != nil {
		a.setError("stop the test to change the configuration")
		return
	}
	a.fields[a.sel].adjust(&a.cfg, d)
	a.message = ""
}

func (a *App) beginEdit() {
	if a.run != nil {
		a.setError("stop the test to change the configuration")
		return
	}
	if a.fields[a.sel].choice {
		a.change(+1)
		return
	}
	a.editing, a.edit = true, ""
	a.setMessage("type a number, Enter to accept, Esc to cancel")
}

// editKey handles a key while the user types a value.
func (a *App) editKey(ev *tcell.EventKey) {
	f := a.fields[a.sel]
	switch ev.Key() {
	case tcell.KeyEscape:
		a.editing = false
		a.setMessage("edit cancelled")
	case tcell.KeyEnter:
		a.editing = false
		if a.edit == "" {
			a.setMessage("edit cancelled")
		} else if err := f.setText(&a.cfg, a.edit); err != nil {
			a.setError(err.Error())
		} else {
			a.setMessage(f.label + " set to " + f.format(f.get(&a.cfg)))
		}
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if a.edit != "" {
			a.edit = a.edit[:len(a.edit)-1]
		}
	case tcell.KeyCtrlC:
		a.editing = false
		a.quit()
	case tcell.KeyRune:
		r := ev.Rune()
		if (r >= '0' && r <= '9' || r == '-' && a.edit == "" && f.min < 0) && len(a.edit) < 9 {
			a.edit += string(r)
		}
	}
}
