package main

// Crash / critical-event logging.
//
// app.log gets a line for every keypress and mouse event, so anything that
// explains a crash (panic stack, fatal signal, stalled pipeline) drowns in
// that noise — and if the process dies before flushing, it is lost entirely.
// Critical events therefore go to a dedicated crash.log (written and closed
// per record, so it survives an abrupt death) and are mirrored into app.log.

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"fyne.io/fyne/v2"
)

const (
	crashLogName      = "crash.log"
	sessionMarkerName = ".micapp.session"
)

var (
	crashMu          sync.Mutex
	sessionStartedAt = time.Now()
	// appShuttingDown suppresses the window watchdog while the app closes normally.
	appShuttingDown int32
)

// appendCrashLine writes one record straight to crash.log — no mutexes and no
// calls into the log package. Used on paths that run while the log package's
// own mutex is held (e.g. a failing stdout write), where any log.Printf would
// deadlock the process.
func appendCrashLine(msg string) {
	if f, err := os.OpenFile(crashLogName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666); err == nil {
		fmt.Fprintf(f, "[%s] %s\n", time.Now().Format("2006-01-02 15:04:05.000"), msg)
		f.Close()
	}
}

// crashf appends a record to crash.log and mirrors it into app.log.
// Safe to call from any goroutine; never panics.
func crashf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	crashMu.Lock()
	// Keep crash.log bounded (rotate at 2MB).
	if fi, err := os.Stat(crashLogName); err == nil && fi.Size() > 2<<20 {
		_ = os.Rename(crashLogName, crashLogName+".old")
	}
	appendCrashLine(msg)
	crashMu.Unlock()
	log.Printf("CRITICAL: %s", strings.ReplaceAll(msg, "\n", "\n    "))
}

// goSafe runs fn in a goroutine; a panic is logged with its full stack
// instead of silently killing the process.
func goSafe(name string, fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				crashf("PANIC in goroutine %q: %v\n%s", name, r, debug.Stack())
			}
		}()
		fn()
	}()
}

func dumpAllGoroutines() string {
	buf := make([]byte, 2<<20)
	n := runtime.Stack(buf, true)
	return string(buf[:n])
}

// installCrashHandlers wires fatal-signal logging, termination logging and an
// on-demand goroutine dump (kill -USR1 <pid>).
func installCrashHandlers() {
	// Make the runtime dump every goroutine on unrecoverable errors.
	debug.SetTraceback("all")

	// A GUI app that logs to stdout dies instantly if that stdout (a pipe or
	// journald socket) goes away: Go raises SIGPIPE for writes to fd 1/2 and
	// the default action kills the process — silently, mid-log-line. Ignore it
	// so a broken stdout only fails the write.
	signal.Ignore(syscall.SIGPIPE)

	fatal := make(chan os.Signal, 1)
	signal.Notify(fatal, syscall.SIGSEGV, syscall.SIGABRT, syscall.SIGBUS, syscall.SIGILL, syscall.SIGFPE)
	goSafe("fatal-signal-handler", func() {
		sig := <-fatal
		crashf("FATAL SIGNAL %v — dumping all goroutine stacks:\n%s", sig, dumpAllGoroutines())
		writeSessionMarker(false)
		signal.Reset(sig)
		if s, ok := sig.(syscall.Signal); ok {
			_ = syscall.Kill(syscall.Getpid(), s) // re-raise so the system still makes a coredump
		}
		os.Exit(2)
	})

	term := make(chan os.Signal, 1)
	signal.Notify(term, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
	goSafe("term-signal-handler", func() {
		sig := <-term
		// Mark first: even if logging is wedged (dead stdout etc.), the marker
		// must say "clean" so the next start does not report a crash.
		writeSessionMarker(true)
		crashf("termination signal %v received — exiting", sig)
		os.Exit(0)
	})

	usr1 := make(chan os.Signal, 1)
	signal.Notify(usr1, syscall.SIGUSR1)
	goSafe("usr1-dump", func() {
		for range usr1 {
			crashf("SIGUSR1: on-demand dump (%s):\n%s", perfState(), dumpAllGoroutines())
		}
	})
}

// writeSessionMarker records whether the process is running / exited cleanly.
// If the next start finds clean=false, the previous run crashed or was killed.
func writeSessionMarker(clean bool) {
	content := fmt.Sprintf("pid=%d\nstarted=%s\nupdated=%s\nclean=%v\n",
		os.Getpid(), sessionStartedAt.Format(time.RFC3339), time.Now().Format(time.RFC3339), clean)
	if err := os.WriteFile(sessionMarkerName, []byte(content), 0666); err != nil {
		log.Printf("session marker write failed: %v", err)
	}
}

// checkPreviousSession reports (loudly) an unclean shutdown of the previous run.
func checkPreviousSession() {
	data, err := os.ReadFile(sessionMarkerName)
	if err != nil {
		return
	}
	s := strings.TrimSpace(string(data))
	if strings.Contains(s, "clean=false") {
		crashf("previous session did not exit cleanly (crashed or was killed):\n%s", indent(s))
	} else {
		log.Printf("previous session exited cleanly")
	}
}

func indent(s string) string {
	return "    " + strings.ReplaceAll(s, "\n", "\n    ")
}

// startWindowWatchdog logs + exits if the process ever ends up with zero
// windows while it is supposed to be running. That state looks exactly like
// "the app closed itself" while the process keeps hanging in the background.
func startWindowWatchdog() {
	goSafe("window-watchdog", func() {
		for {
			time.Sleep(10 * time.Second)
			if atomic.LoadInt32(&appShuttingDown) == 1 {
				return
			}
			app := fyne.CurrentApp()
			if app == nil {
				continue
			}
			if n := len(app.Driver().AllWindows()); n == 0 {
				crashf("WATCHDOG: no windows left while the process is alive (main window destroyed without a normal exit) — exiting with code 3")
				writeSessionMarker(false)
				os.Exit(3)
			}
		}
	})
}
