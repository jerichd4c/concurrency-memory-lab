package ui

import (
	"fmt"
	"os"
	"strings"

	"concurrency-memory-lab/runner"
)

// Lang is a language of the interface. Only what is shown on screen
// changes: the code, the worker output and the names Linux uses (cpu.max,
// RSS, swap, goroutine, mutex...) stay the same in both.
type Lang int

const (
	English Lang = iota
	Spanish
)

// ParseLang converts the value of the -lang flag. "" means: take it from
// the environment.
func ParseLang(s string) (Lang, error) {
	switch strings.ToLower(s) {
	case "":
		return LangFromEnv(), nil
	case "en", "english":
		return English, nil
	case "es", "spanish", "español":
		return Spanish, nil
	}
	return English, fmt.Errorf("unknown language %q (use en or es)", s)
}

// LangFromEnv returns Spanish when the locale of the user is Spanish
// (LC_ALL, LC_MESSAGES or LANG starting with "es"), English otherwise.
func LangFromEnv() Lang {
	for _, name := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := os.Getenv(name); v != "" {
			if strings.HasPrefix(v, "es") {
				return Spanish
			}
			return English
		}
	}
	return English
}

// text holds every text of the interface in one language. Names ending in
// F are fmt formats. The lengths matter: the layout is 80x24, and
// TestTextsFit checks that every text fits where it is drawn.
type text struct {
	switchLang string // line 2, right: how to change the language

	title, limitsOK, limitsNotF, notRoot string
	tooSmall, tooSmallF, quitHint        string
	keysMain, keysCompare                string

	// Form.
	configuration, none, noSwap, notNumberF, betweenF string
	labels, helps                                     [9]string
	syncLabels                                        [4]string

	// Live panel.
	live, idle, safeF, runningF                                 string
	virtual, resident, swap, faults, faultsF, threads, threadsF string
	cgroupMemory, throttled, throttledF, cgroup, cpuLimitF      string
	limitNames                                                  map[string]string
	applied, notApplied                                         string

	// Results.
	lastResultF, testF, counterWord, noSwapMachine string
	outcomes, outcomesShort                        map[runner.Outcome]string
	workUnfinished, workF, counterF                string
	peakF, cpuF, cgroupF, noCgroup                 string
	limitsNone, limitsAll, limitsNotAll, limitsTo  string

	// Compare view.
	tableHeader, legend, noResults string

	// Messages.
	alreadyRunning, cannotStart, started, noFinished, notRunning   string
	stopping, finishedF, stoppingQuit, orphansErr, orphansF        string
	defaults, locked, typeHint, editCancelled, setToF, langChanged string
	unsafeRAMF, unsafeSwapF                                        string
}

var english = text{
	switchLang: "l: cambiar a español",

	title:       " concurrency-memory-lab  -  threads, CPU limits, virtual memory and swap",
	limitsOK:    "limits: available (root, cgroup v2)",
	limitsNotF:  "limits NOT available: %s - tests run without limits",
	notRoot:     "not root, start it with sudo",
	tooSmall:    "terminal too small",
	tooSmallF:   "%dx%d, needs %dx%d",
	quitHint:    "q quits",
	keysMain:    " arrows move/change  Enter type  s start  x stop  r repeat  c compare  q quit",
	keysCompare: " up/down select  r repeat selected  s start  x stop  c/Esc back  q quit",

	configuration: "CONFIGURATION",
	none:          "none",
	noSwap:        "0 (no swap)",
	notNumberF:    "%q is not a number",
	betweenF:      "%s must be between %d and %d",
	labels: [9]string{"Threads", "Worker type", "Memory to use", "CPU work", "Shared counter",
		"CPU limit", "Memory limit", "Swap limit", "Time limit"},
	helps: [9]string{
		"workers that share the CPU work",
		"goroutine: tasks of the Go runtime (M:N)  os-thread: one OS thread each (1:1)",
		"MiB the worker reserves with mmap and then touches page by page",
		"millions of iterations (or of counter increments), split among the threads",
		"off: independent work  none/mutex/atomic: all threads add 1 to one counter",
		"percent of ONE CPU the test may use (cpu.max); 200 = two CPUs; none = no limit",
		"most RAM the test may use (memory.max), in MiB; none = no limit",
		"most swap the test may use (memory.swap.max), in MiB; -1 = none, 0 = no swap",
		"seconds before the test is stopped; none = never",
	},
	syncLabels: [4]string{"off", "no sync", "mutex", "atomic"},

	live:         "LIVE",
	idle:         "idle - press s to start a test",
	safeF:        "safe now: %d MiB RAM, %d MiB swap",
	runningF:     "running %.1fs  worker pid %d",
	virtual:      "Virtual memory",
	resident:     "Resident (RAM)",
	swap:         "Swap",
	faults:       "Page faults",
	faultsF:      "%d min %d maj",
	threads:      "Threads",
	threadsF:     "%-6d CPU time %v",
	cgroupMemory: "Cgroup memory",
	throttled:    "CPU throttled",
	throttledF:   "%d/%d periods",
	cgroup:       "Cgroup",
	cpuLimitF:    "%d%% of one CPU",
	limitNames:   map[string]string{"cpu": "cpu", "memory": "memory", "swap": "swap"},
	applied:      "applied",
	notApplied:   "NOT applied",

	lastResultF:   "LAST RESULT  test %d: %s",
	testF:         "TEST %d: %s",
	counterWord:   "counter",
	noSwapMachine: "no swap on this machine",
	outcomes: map[runner.Outcome]string{
		runner.OutcomeCompleted: "completed",
		runner.OutcomeStopped:   "stopped by the user",
		runner.OutcomeTimeout:   "stopped: time limit reached",
		runner.OutcomeOOM:       "killed by the kernel: out of memory in the cgroup",
		runner.OutcomeKilled:    "killed by a signal",
		runner.OutcomeFailed:    "failed",
	},
	outcomesShort: map[runner.Outcome]string{
		runner.OutcomeCompleted: "ok",
		runner.OutcomeStopped:   "stopped",
		runner.OutcomeTimeout:   "timeout",
		runner.OutcomeOOM:       "OOM",
		runner.OutcomeKilled:    "killed",
		runner.OutcomeFailed:    "failed",
	},
	workUnfinished: "work: not finished",
	workF:          "work: %d iterations in %v",
	counterF:       "shared counter: %d of %d increments, lost %d (%.1f%%) in %v",
	peakF:          "peak RAM %s  peak swap %s  faults %d minor %d major",
	cpuF:           "cpu %.2fs user %.2fs sys  ",
	cgroupF:        "throttled %d/%d  oom %d  ",
	noCgroup:       "no cgroup  ",
	limitsNone:     "limits: none requested",
	limitsAll:      "limits: all applied",
	limitsNotAll:   "limits: NOT applied",
	limitsTo:       "limits: ",

	tableHeader: fmt.Sprintf(headerFormat,
		"#", "thread", "sync", "mem", "work", "limits", "result", "time", "RAM", "swap", "majfl", "lost"),
	legend:    "time: whole test  RAM/swap: peak MiB  * = limits asked but NOT applied",
	noResults: "no test has finished yet: run some tests, then compare them here",

	alreadyRunning: "a test is already running",
	cannotStart:    "cannot start: ",
	started:        "test started",
	noFinished:     "no test has finished yet",
	notRunning:     "no test is running",
	stopping:       "stopping the test...",
	finishedF:      "test finished: %s",
	stoppingQuit:   "stopping the test before quitting...",
	orphansErr:     "could not remove old cgroups: ",
	orphansF:       "removed %d cgroup(s) left by an earlier run",
	defaults:       "default configuration",
	locked:         "stop the test to change the configuration",
	typeHint:       "type a number, Enter to accept, Esc to cancel",
	editCancelled:  "edit cancelled",
	setToF:         "%s set to %s",
	langChanged:    "language: English",
	unsafeRAMF:     "unsafe: %d MiB in RAM, safe max %d MiB (half of available RAM)",
	unsafeSwapF:    "unsafe: %d MiB to swap, safe max %d MiB (half of free swap)",
}

var spanish = text{
	switchLang: "l: switch to English",

	title:       " concurrency-memory-lab  -  hilos, límites de CPU, memoria virtual y swap",
	limitsOK:    "límites: disponibles (root, cgroup v2)",
	limitsNotF:  "límites NO disponibles: %s; se prueba sin límites",
	notRoot:     "no es root, ábrelo con sudo",
	tooSmall:    "terminal muy chica",
	tooSmallF:   "%dx%d, necesita %dx%d",
	quitHint:    "q sale",
	keysMain:    " flechas mover  Enter edita  s inicia  x para  r repite  c compara  q sale",
	keysCompare: " arriba/abajo elegir  r repite elegida  s inicia  x para  c/Esc volver  q sale",

	configuration: "CONFIGURACIÓN",
	none:          "ninguno",
	noSwap:        "0 (sin swap)",
	notNumberF:    "%q no es un número",
	betweenF:      "%s debe estar entre %d y %d",
	labels: [9]string{"Hilos", "Tipo de hilo", "Memoria a usar", "Trabajo de CPU", "Contador común",
		"Límite de CPU", "Límite memoria", "Límite de swap", "Límite tiempo"},
	helps: [9]string{
		"hilos de trabajo que se reparten el trabajo de CPU",
		"goroutine: tareas del runtime de Go (M:N)  os-thread: hilo propio del SO (1:1)",
		"MiB que el worker reserva con mmap y luego toca página por página",
		"millones de iteraciones (o incrementos del contador), repartidos entre hilos",
		"no: trabajo aparte  sin sincronizar/mutex/atomic: todos suman 1 a un contador",
		"porcentaje de UNA CPU que puede usar la prueba (cpu.max); 200 = dos CPUs",
		"RAM máxima de la prueba (memory.max), en MiB; ninguno = sin límite",
		"swap máxima (memory.swap.max), en MiB; -1 = sin límite, 0 = sin swap",
		"segundos antes de detener la prueba; ninguno = nunca",
	},
	syncLabels: [4]string{"no", "sin sincronizar", "mutex", "atomic"},

	live:         "EN VIVO",
	idle:         "en espera - pulsa s para iniciar",
	safeF:        "seguro: %d MiB RAM, %d MiB swap",
	runningF:     "corriendo %.1fs  pid worker %d",
	virtual:      "Memoria virtual",
	resident:     "Residente (RAM)",
	swap:         "Swap",
	faults:       "Fallos de pág.",
	faultsF:      "%d men %d may",
	threads:      "Hilos",
	threadsF:     "%-6d CPU %v",
	cgroupMemory: "Memoria cgroup",
	throttled:    "CPU frenada",
	throttledF:   "%d/%d periodos",
	cgroup:       "Cgroup",
	cpuLimitF:    "%d%% de una CPU",
	limitNames:   map[string]string{"cpu": "cpu", "memory": "memoria", "swap": "swap"},
	applied:      "aplicado",
	notApplied:   "NO aplicado",

	lastResultF:   "ÚLTIMO RESULTADO  prueba %d: %s",
	testF:         "PRUEBA %d: %s",
	counterWord:   "contador",
	noSwapMachine: "esta máquina no tiene swap",
	outcomes: map[runner.Outcome]string{
		runner.OutcomeCompleted: "completada",
		runner.OutcomeStopped:   "detenida por el usuario",
		runner.OutcomeTimeout:   "detenida: se alcanzó el límite de tiempo",
		runner.OutcomeOOM:       "terminada por el kernel: sin memoria en el cgroup (OOM)",
		runner.OutcomeKilled:    "terminada por una señal",
		runner.OutcomeFailed:    "falló",
	},
	outcomesShort: map[runner.Outcome]string{
		runner.OutcomeCompleted: "ok",
		runner.OutcomeStopped:   "parada",
		runner.OutcomeTimeout:   "tiempo",
		runner.OutcomeOOM:       "OOM",
		runner.OutcomeKilled:    "señal",
		runner.OutcomeFailed:    "falló",
	},
	workUnfinished: "trabajo: sin terminar",
	workF:          "trabajo: %d iteraciones en %v",
	counterF:       "contador: %d de %d incrementos, perdidos %d (%.1f%%) en %v",
	peakF:          "pico RAM %s  pico swap %s  fallos %d menores %d mayores",
	cpuF:           "cpu %.2fs usr %.2fs sis  ",
	cgroupF:        "frenada %d/%d  oom %d  ",
	noCgroup:       "sin cgroup  ",
	limitsNone:     "límites: ninguno",
	limitsAll:      "límites: aplicados",
	limitsNotAll:   "límites: NO aplicados",
	limitsTo:       "límites: ",

	tableHeader: fmt.Sprintf(headerFormat,
		"#", "hilos", "sinc.", "mem", "trab.", "límites", "estado", "tiempo", "RAM", "swap", "f.may", "perd."),
	legend:    "tiempo: prueba entera  RAM/swap: pico MiB  * = límites NO aplicados",
	noResults: "todavía no terminó ninguna prueba: corre algunas y compáralas aquí",

	alreadyRunning: "ya hay una prueba corriendo",
	cannotStart:    "no se puede iniciar: ",
	started:        "prueba iniciada",
	noFinished:     "todavía no terminó ninguna prueba",
	notRunning:     "no hay ninguna prueba corriendo",
	stopping:       "deteniendo la prueba...",
	finishedF:      "prueba terminada: %s",
	stoppingQuit:   "deteniendo la prueba antes de salir...",
	orphansErr:     "no se pudieron borrar cgroups viejos: ",
	orphansF:       "se borraron %d cgroup(s) de una ejecución anterior",
	defaults:       "configuración por defecto",
	locked:         "detén la prueba para cambiar la configuración",
	typeHint:       "escribe un número, Enter acepta, Esc cancela",
	editCancelled:  "edición cancelada",
	setToF:         "%s quedó en %s",
	langChanged:    "idioma: español",
	unsafeRAMF:     "inseguro: %d MiB en RAM, el máximo es %d MiB (mitad de la RAM disponible)",
	unsafeSwapF:    "inseguro: %d MiB a swap, el máximo es %d MiB (mitad de la swap libre)",
}

// texts returns the texts of a language.
func texts(l Lang) *text {
	if l == Spanish {
		return &spanish
	}
	return &english
}

// unsafeText explains in the language of the screen why a test was refused.
func (t *text) unsafeText(e *runner.UnsafeError) string {
	if e.Swap {
		return fmt.Sprintf(t.unsafeSwapF, e.MiB, e.MaxMiB)
	}
	return fmt.Sprintf(t.unsafeRAMF, e.MiB, e.MaxMiB)
}
