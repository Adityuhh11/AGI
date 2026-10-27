package sidecar

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func newSwitch(t *testing.T, scripts ...script) (*Switch, *fakeEngine, *Supervisor) {
	t.Helper()
	engine := &fakeEngine{scripts: scripts}
	sup, err := New(Options{
		Dir:            filepath.Join("C:", "engine", "app"),
		Python:         filepath.Join("C:", "engine", "venv", "Scripts", "python.exe"),
		Start:          engine.start,
		Backoff:        func(int) time.Duration { return 2 * time.Millisecond },
		Poll:           5 * time.Millisecond,
		StartupTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	w := NewSwitch(ctx, sup)
	t.Cleanup(func() { cancel(); w.Wait() })
	return w, engine, sup
}

func waitFor(t *testing.T, sup *Supervisor, want State) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(2 * time.Millisecond) {
		if sup.Status().State == want {
			return
		}
	}
	t.Fatalf("never reached %q, now %+v", want, sup.Status())
}

func TestNothingRunsUntilTheSwitchIsTurnedOn(t *testing.T) {
	w, engine, sup := newSwitch(t, script{})
	w.Off()
	if got := sup.Status().State; got != StateOff || engine.starts() != 0 {
		t.Fatalf("state %q after %d starts, want off after none", got, engine.starts())
	}
}

func TestOnStartsTheEngineAndOffStopsItAndItCanComeBack(t *testing.T) {
	w, engine, sup := newSwitch(t, script{})
	w.On()
	w.On() // asking again changes nothing
	waitFor(t, sup, StateReady)
	w.Off()
	if sup.Status().State != StateOff || engine.killed() != 1 || engine.starts() != 1 {
		t.Fatalf("after Off: %+v, %d kills, %d starts", sup.Status(), engine.killed(), engine.starts())
	}
	w.On()
	waitFor(t, sup, StateReady)
	if engine.starts() != 2 {
		t.Fatalf("started %d times, want 2", engine.starts())
	}
}

func TestAnEngineThatGaveUpIsStartedAgainByOn(t *testing.T) {
	w, engine, sup := newSwitch(t,
		script{output: "omni_memory: bad\n", exitAfter: 5 * time.Millisecond, exitCode: 2}, script{})
	w.On()
	waitFor(t, sup, StateFailed)
	w.On()
	waitFor(t, sup, StateReady)
	if engine.starts() != 2 {
		t.Fatalf("started %d times, want 2", engine.starts())
	}
}
