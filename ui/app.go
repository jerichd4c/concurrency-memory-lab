// Package ui is the terminal interface of the lab: a form to configure the
// test, a live panel while it runs and the result when it ends.
package ui

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gdamore/tcell/v2"

	"resource-lab/cgroup"
	"resource-lab/metrics"
	"resource-lab/runner"
)

// sampleInterval is the time between two readings of the running test.
const sampleInterval = 500 * time.Millisecond

// App holds the state of the interface. Only the main loop touches it, so
// it needs no locks.
type App struct {
	screen tcell.Screen
	exe    string // this program, started again in worker mode

	cfg     runner.Config // what the next test will use
	fields  []field
	sel     int    // selected field
	editing bool   // the user is typing a value for the selected field
	edit    string // what has been typed so far

	run       *runner.Run   // the test that is running, nil when idle
	sample    runner.Sample // last reading of the running test
	hasSample bool
	results   []runner.Result // every finished test, oldest first

	limitsErr error          // why limits cannot be applied, nil if they can
	system    metrics.System // RAM and swap of the machine
	message   string         // result of the last action
	msgIsErr  bool
	quitting  bool
	start     func(runner.Config) (*runner.Run, error) // runner.Start, replaced in tests
}

// Run starts the interface and returns when the user quits. exe is the path
// of this program, used to start the worker.
func Run(exe string) error {
	screen, err := tcell.NewScreen()
	if err != nil {
		return err
	}
	if err := screen.Init(); err != nil {
		return err
	}
	a := newApp(screen, exe)
	a.loop()
	return nil
}

func newApp(screen tcell.Screen, exe string) *App {
	a := &App{
		screen: screen,
		exe:    exe,
		cfg:    defaultConfig(),
		fields: newFields(),
		start:  func(c runner.Config) (*runner.Run, error) { return runner.Start(exe, c) },
	}
	a.limitsErr = limitsAvailable()
	a.system, _ = metrics.ReadSystem()
	return a
}

// limitsAvailable tells, before any test, whether limits can be applied:
// cgroup v2 must be there and only root may create cgroups.
func limitsAvailable() error {
	if err := cgroup.Available(); err != nil {
		return err
	}
	if os.Geteuid() != 0 {
		return errNotRoot
	}
	return nil
}

// loop draws the screen and then waits for a key, a sample tick, the end of
// the test or a signal, whichever comes first.
func (a *App) loop() {
	// Restore the terminal however we leave, even after a panic. The worker
	// dies with us anyway (Pdeathsig).
	defer func() {
		p := recover()
		a.screen.Fini()
		if p != nil {
			panic(p)
		}
	}()
	a.screen.HideCursor()

	events := make(chan tcell.Event)
	go func() {
		for {
			ev := a.screen.PollEvent()
			if ev == nil {
				return // the screen was closed
			}
			events <- ev
		}
	}()
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	defer signal.Stop(signals)
	ticker := time.NewTicker(sampleInterval)
	defer ticker.Stop()

	for !a.quitting || a.run != nil {
		a.draw()
		var done <-chan struct{} // nil while idle: that case never fires
		if a.run != nil {
			done = a.run.Done()
		}
		select {
		case ev := <-events:
			a.handleEvent(ev)
		case <-ticker.C:
			a.takeSample()
		case <-done:
			a.finish()
		case <-signals:
			a.quit()
		}
	}
}

// startTest starts a test with the current configuration.
func (a *App) startTest() {
	if a.run != nil {
		a.setError("a test is already running")
		return
	}
	r, err := a.start(a.cfg)
	if err != nil {
		a.setError("cannot start: " + err.Error())
		return
	}
	a.run, a.hasSample = r, false
	a.setMessage("test started")
	a.takeSample()
}

// repeatTest starts again the configuration of the last finished test.
func (a *App) repeatTest() {
	if len(a.results) == 0 {
		a.setError("no test has finished yet")
		return
	}
	a.cfg = a.results[len(a.results)-1].Config
	a.startTest()
}

func (a *App) stopTest() {
	if a.run == nil {
		a.setError("no test is running")
		return
	}
	a.run.Stop()
	a.setMessage("stopping the test...")
}

// takeSample reads the running test. A failed reading keeps the last one:
// it happens when the worker has just exited.
func (a *App) takeSample() {
	if a.run == nil {
		return
	}
	if s, err := a.run.Sample(); err == nil {
		a.sample, a.hasSample = s, true
	}
}

// finish stores the result of the test that just ended.
func (a *App) finish() {
	res := a.run.Result()
	a.results = append(a.results, res)
	a.run = nil
	a.setMessage("test finished: " + string(res.Outcome))
	if a.quitting {
		return
	}
	a.system, _ = metrics.ReadSystem()
}

// quit leaves at once when idle; with a test running it stops it first and
// the loop ends when the test is done.
func (a *App) quit() {
	a.quitting = true
	if a.run != nil {
		a.run.Stop()
		a.setMessage("stopping the test before quitting...")
	}
}

func (a *App) setMessage(m string) { a.message, a.msgIsErr = m, false }
func (a *App) setError(m string)   { a.message, a.msgIsErr = m, true }
