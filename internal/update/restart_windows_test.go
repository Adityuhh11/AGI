package update

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"omni/internal/sidecar"
)

// TestMain lets this test program play both launchers of an update. Started with "--wait-for <pid>" it is the new
// launcher: it waits for that process to end and leaves a note. Started with OMNI_TEST_ROLE=old-launcher it is the one
// being replaced: it puts itself in the launcher's kill-on-close job (as main does), starts its successor, and exits at once.
func TestMain(m *testing.M) {
	if pid, ok := WaitForArg(os.Args[1:]); ok {
		if !WaitForExit(pid, 10*time.Second) {
			os.Exit(4)
		}
		if err := os.WriteFile(os.Getenv("OMNI_TEST_MARKER"), []byte("started after the old launcher had ended"), 0o644); err != nil {
			os.Exit(5)
		}
		os.Exit(0)
	}
	if os.Getenv("OMNI_TEST_ROLE") == "old-launcher" {
		if err := sidecar.KillWithLauncher(); err != nil {
			fmt.Fprintln(os.Stderr, "job:", err)
			os.Exit(2)
		}
		exe, _ := os.Executable()
		var err error
		if os.Getenv("OMNI_TEST_MODE") == "plain" { // the control: an ordinary child, which joins the job
			err = exec.Command(exe, WaitForFlag, strconv.Itoa(os.Getpid())).Start()
		} else {
			err = StartNew(exe, os.Getpid())
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "start:", err)
			os.Exit(3)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func runOldLauncher(t *testing.T, mode string) (marker string) {
	t.Helper()
	marker = filepath.Join(t.TempDir(), "marker.txt")
	cmd := exec.Command(os.Args[0])
	cmd.Env = append(os.Environ(), "OMNI_TEST_ROLE=old-launcher", "OMNI_TEST_MODE="+mode, "OMNI_TEST_MARKER="+marker)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("the old launcher: %v\n%s", err, out)
	}
	return marker
}

func appears(path string, within time.Duration) bool {
	for deadline := time.Now().Add(within); time.Now().Before(deadline); time.Sleep(25 * time.Millisecond) {
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}
	return false
}

func TestTheNewLauncherOutlivesTheOneThatStartedItEvenInsideTheKillOnCloseJob(t *testing.T) {
	marker := runOldLauncher(t, "breakaway")
	if !appears(marker, 8*time.Second) {
		t.Fatal("the new launcher never got to start: it died with the old one's job, or never waited for it")
	}
}

// The control: without the breakaway the same child is killed with the job, so the test above would notice.
func TestAChildStartedTheOrdinaryWayDiesWithTheLauncherSoTheTestIsReal(t *testing.T) {
	marker := runOldLauncher(t, "plain")
	if appears(marker, 3*time.Second) {
		t.Fatal("an ordinary child survived the job: the breakaway test above proves nothing here")
	}
}

func TestWaitForExitSeesAnEndedProcessAndTimesOutOnARunningOne(t *testing.T) {
	quick := exec.Command("cmd", "/c", "exit 0")
	if err := quick.Start(); err != nil {
		t.Fatal(err)
	}
	pid := quick.Process.Pid
	_ = quick.Wait()
	if !WaitForExit(pid, 2*time.Second) {
		t.Error("a process that ended was reported as still running")
	}

	slow := exec.Command("ping", "-n", "30", "127.0.0.1")
	if err := slow.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = slow.Process.Kill(); _ = slow.Wait() }()
	start := time.Now()
	if WaitForExit(slow.Process.Pid, 300*time.Millisecond) {
		t.Error("a running process was reported as ended")
	}
	if took := time.Since(start); took < 250*time.Millisecond || took > 3*time.Second {
		t.Errorf("waited %v for a 300 ms timeout", took)
	}
}

func TestWaitForExitReturnsAtOnceWhenTheProcessEndsDuringTheWait(t *testing.T) {
	slow := exec.Command("ping", "-n", "30", "127.0.0.1")
	if err := slow.Start(); err != nil {
		t.Fatal(err)
	}
	go func() { time.Sleep(200 * time.Millisecond); _ = slow.Process.Kill() }()
	defer slow.Wait()
	start := time.Now()
	if !WaitForExit(slow.Process.Pid, 10*time.Second) {
		t.Fatal("the process ended and the wait did not notice")
	}
	if time.Since(start) > 3*time.Second {
		t.Fatal("the wait did not return when the process ended")
	}
}

func TestAProcessThatNeverExistedCountsAsEnded(t *testing.T) {
	if !WaitForExit(4_000_000, time.Second) {
		t.Fatal("a process id nobody has was treated as running")
	}
}
