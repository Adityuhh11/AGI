package settingsui

import (
	"errors"
	"strings"
	"testing"

	"agi/internal/update"
)

// fakeInstaller plays the update installer: it records what it was asked to install and lets the test finish it.
type fakeInstaller struct {
	started  []update.Asset
	exe      string
	done     func(error)
	startErr error
	cancels  int
	progress update.Progress
}

func (f *fakeInstaller) Start(asset update.Asset, exe string, done func(error)) error {
	if f.startErr != nil {
		return f.startErr
	}
	f.started, f.exe, f.done = append(f.started, asset), exe, done
	return nil
}
func (f *fakeInstaller) Cancel()                   { f.cancels++ }
func (f *fakeInstaller) Progress() update.Progress { return f.progress }

var releaseWithZip = update.Release{
	Version: "0.4.0", URL: "https://github.com/Adityuhh11/AGI/releases/tag/v0.4.0",
	Asset: update.Asset{Name: "agi-0.4.0-windows-x64.zip", URL: "https://github.com/Adityuhh11/AGI/releases/download/v0.4.0/agi-0.4.0-windows-x64.zip", Size: 5000, SHA256: strings.Repeat("a", 64)},
}

// foundRelease makes the last check have found rel.
func (h *harness) foundRelease(rel update.Release) {
	h.updates.rel, h.updates.newer = rel, true
	h.svc.CheckUpdate()
}

func TestStartingAnUpdateInstallsTheFoundReleaseOverThisProgram(t *testing.T) {
	h := newHarness(t)
	h.foundRelease(releaseWithZip)
	if msg := h.svc.StartUpdate(); msg != "" {
		t.Fatal(msg)
	}
	if len(h.installer.started) != 1 || h.installer.started[0] != releaseWithZip.Asset || h.installer.exe != "C:/app/agi.exe" {
		t.Fatalf("installer asked for %+v over %q", h.installer.started, h.installer.exe)
	}
	if h.restarts != 0 {
		t.Fatal("restarted before the install had finished")
	}
}

func TestNothingIsInstalledWithoutAFoundReleaseOrAZipThatCanBeChecked(t *testing.T) {
	h := newHarness(t)
	if msg := h.svc.StartUpdate(); !strings.Contains(msg, "no newer release") {
		t.Fatalf("with nothing found: %q", msg)
	}
	noZip := releaseWithZip
	noZip.Asset = update.Asset{}
	h.foundRelease(noZip)
	if msg := h.svc.StartUpdate(); !strings.Contains(msg, "release page") {
		t.Fatalf("with no checkable zip: %q (it must point to the release page)", msg)
	}
	if len(h.installer.started) != 0 {
		t.Fatalf("installer started: %+v", h.installer.started)
	}
}

func TestTheLauncherRestartsOnlyAfterASuccessfulInstall(t *testing.T) {
	h := newHarness(t)
	h.foundRelease(releaseWithZip)
	h.svc.StartUpdate()
	h.installer.done(errors.New("the download does not match its checksum"))
	if h.restarts != 0 {
		t.Fatal("restarted after a failed install")
	}
	h.svc.StartUpdate()
	h.installer.done(nil)
	if h.restarts != 1 {
		t.Fatalf("restarted %d times after a good install", h.restarts)
	}
}

func TestARestartThatCannotHappenIsToldToThePageOnceTheInstallIsDone(t *testing.T) {
	h := newHarness(t)
	h.restartErr = errors.New("access is denied")
	h.foundRelease(releaseWithZip)
	h.svc.StartUpdate()
	h.installer.done(nil)
	h.installer.progress = update.Progress{Stage: update.StageDone}
	got := h.svc.UpdateProgress()
	if got.Stage != "done" || !strings.Contains(got.Error, "access is denied") || !strings.Contains(got.Error, "start it again") {
		t.Fatalf("page sees %+v", got)
	}
	h.installer.progress = update.Progress{Stage: update.StageDownloading, Done: 1}
	if got := h.svc.UpdateProgress(); got.Error != "" {
		t.Fatalf("a restart error from before showed on a new download: %+v", got)
	}
	h.restartErr = nil
	h.svc.StartUpdate()
	if got := h.svc.UpdateProgress(); got.Error != "" {
		t.Fatalf("starting again did not clear the old restart error: %+v", got)
	}
}

func TestAnInstallThatCannotStartSaysWhy(t *testing.T) {
	h := newHarness(t)
	h.foundRelease(releaseWithZip)
	h.installer.startErr = errors.New("an update is already being installed")
	if msg := h.svc.StartUpdate(); !strings.Contains(msg, "already being installed") {
		t.Fatalf("message %q", msg)
	}
}

func TestProgressAndCancelGoThroughToTheInstaller(t *testing.T) {
	h := newHarness(t)
	h.installer.progress = update.Progress{Stage: update.StageDownloading, Done: 2048, Total: 5000}
	if got := h.svc.UpdateProgress(); got.Stage != "downloading" || got.Done != 2048 || got.Total != 5000 || got.Error != "" {
		t.Fatalf("progress %+v", got)
	}
	h.installer.progress = update.Progress{Stage: update.StageFailed, Error: "no network"}
	if got := h.svc.UpdateProgress(); got.Stage != "failed" || got.Error != "no network" {
		t.Fatalf("progress %+v", got)
	}
	h.svc.CancelUpdate()
	if h.installer.cancels != 1 {
		t.Fatalf("cancelled %d times", h.installer.cancels)
	}
}

func TestThePageIsOnlyToldItCanInstallWhenTheReleaseHasAZipThatCanBeChecked(t *testing.T) {
	h := newHarness(t)
	h.updates.rel, h.updates.newer = releaseWithZip, true
	if got := h.svc.CheckUpdate(); !got.Available || !got.Installable {
		t.Fatalf("a release with a zip: %+v", got)
	}
	if v := h.svc.View(); v.UpdateFound != "0.4.0" || !v.UpdateInstallable {
		t.Fatalf("the page on opening: found %q, installable %v", v.UpdateFound, v.UpdateInstallable)
	}
	noZip := releaseWithZip
	noZip.Asset = update.Asset{}
	h.updates.rel = noZip
	if got := h.svc.CheckUpdate(); !got.Available || got.Installable {
		t.Fatalf("a release without a checkable zip: %+v (it is available, but the page must not offer to install it)", got)
	}
	if v := h.svc.View(); v.UpdateFound != "0.4.0" || v.UpdateInstallable {
		t.Fatalf("the page on opening: found %q, installable %v", v.UpdateFound, v.UpdateInstallable)
	}
	h.updates.newer = false
	if got := h.svc.CheckUpdate(); got.Installable {
		t.Fatalf("an old release is not installable: %+v", got)
	}
}
