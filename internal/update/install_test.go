package update

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

var (
	oldProgram = []byte("MZ the program that is running")
	newProgram = []byte("MZ the program that was downloaded")
)

type zipEntry struct {
	name string
	data []byte
}

func zipOf(t *testing.T, entries ...zipEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, e := range entries {
		f, err := w.Create(e.name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write(e.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func releaseZip(t *testing.T) []byte {
	return zipOf(t, zipEntry{"omni.exe", newProgram}, zipEntry{"README.txt", []byte("hello")})
}

func sumOf(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }

// rig is a program folder with a running program in it, a server holding a release, and an installer pointed at both.
type rig struct {
	t      *testing.T
	dir    string // where the program is
	exe    string
	work   string
	srv    *httptest.Server
	hits   atomic.Int32
	inst   *Installer
	asset  Asset
	handle func(w http.ResponseWriter, r *http.Request)
}

func newRig(t *testing.T, body []byte) *rig {
	t.Helper()
	root := t.TempDir()
	r := &rig{t: t, dir: filepath.Join(root, "app"), work: filepath.Join(root, "work")}
	if err := os.MkdirAll(r.dir, 0o755); err != nil {
		t.Fatal(err)
	}
	r.exe = filepath.Join(r.dir, "omni.exe")
	if err := os.WriteFile(r.exe, oldProgram, 0o755); err != nil {
		t.Fatal(err)
	}
	r.handle = func(w http.ResponseWriter, _ *http.Request) { w.Write(body) }
	r.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		r.hits.Add(1)
		r.handle(w, req)
	}))
	t.Cleanup(r.srv.Close)
	r.inst = NewInstaller(r.work)
	r.inst.prefix = r.srv.URL + "/"
	r.inst.sleep = func(time.Duration) {}
	r.asset = Asset{Name: "omni-9.9.9-windows-x64.zip", URL: r.srv.URL + "/v9.9.9/omni-9.9.9-windows-x64.zip", Size: int64(len(body)), SHA256: sumOf(body)}
	return r
}

func (r *rig) install() error { return r.inst.Install(context.Background(), r.asset, r.exe) }

// untouched says the folder still holds exactly the running program and nothing else, and the download folder is gone.
func (r *rig) untouched() {
	r.t.Helper()
	got, err := os.ReadFile(r.exe)
	if err != nil || !bytes.Equal(got, oldProgram) {
		r.t.Fatalf("the running program changed: %q, %v", got, err)
	}
	r.onlyThe([]string{"omni.exe"})
	if _, err := os.Stat(r.work); err == nil {
		r.t.Fatal("the download folder was left behind")
	}
}

func (r *rig) onlyThe(names []string) {
	r.t.Helper()
	entries, err := os.ReadDir(r.dir)
	if err != nil {
		r.t.Fatal(err)
	}
	var got []string
	for _, e := range entries {
		got = append(got, e.Name())
	}
	slices.Sort(got)
	slices.Sort(names)
	if !slices.Equal(got, names) {
		r.t.Fatalf("the program's folder holds %v, want %v", got, names)
	}
}

func (r *rig) failedWith(err error, want string) {
	r.t.Helper()
	if err == nil || !strings.Contains(err.Error(), want) {
		r.t.Fatalf("error %v, want one containing %q", err, want)
	}
	if p := r.inst.Progress(); p.Stage != StageFailed || p.Error == "" {
		r.t.Fatalf("progress %+v, want failed with a reason", p)
	}
}

func TestAnUpdateReplacesTheProgramAndKeepsTheOldOneAside(t *testing.T) {
	r := newRig(t, releaseZip(t))
	if err := r.install(); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(r.exe); !bytes.Equal(got, newProgram) {
		t.Fatalf("the program is %q", got)
	}
	if got, _ := os.ReadFile(r.exe + ".old"); !bytes.Equal(got, oldProgram) {
		t.Fatalf("the old program is %q", got)
	}
	r.onlyThe([]string{"omni.exe", "omni.exe.old"})
	if _, err := os.Stat(r.work); err == nil {
		t.Error("the download folder was left behind")
	}
	if p := r.inst.Progress(); p.Stage != StageDone {
		t.Errorf("progress %+v", p)
	}
}

func TestADownloadReportsItsBytesInOrderAndEndsAtTheFullSize(t *testing.T) {
	body := releaseZip(t)
	r := newRig(t, body)
	r.handle = func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "")
		for i := 0; i < len(body); i += 40 {
			w.Write(body[i:min(i+40, len(body))])
			w.(http.Flusher).Flush()
		}
	}
	if err := os.MkdirAll(r.work, 0o755); err != nil {
		t.Fatal(err)
	}
	var seen []int64
	var total int64
	if _, err := r.inst.download(context.Background(), r.asset, r.work, func(done, tot int64) { seen = append(seen, done); total = tot }); err != nil {
		t.Fatal(err)
	}
	if len(seen) < 2 || !slices.IsSorted(seen) || seen[len(seen)-1] != int64(len(body)) || total != int64(len(body)) {
		t.Fatalf("progress %v of %d for %d bytes", seen, total, len(body))
	}
}

func TestADownloadThatDoesNotMatchItsChecksumInstallsNothing(t *testing.T) {
	r := newRig(t, releaseZip(t))
	r.asset.SHA256 = sumOf([]byte("something else"))
	r.failedWith(r.install(), "checksum")
	r.untouched()
}

func TestATruncatedDownloadInstallsNothing(t *testing.T) {
	body := releaseZip(t)
	r := newRig(t, body)
	r.handle = func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "100000")
		w.Write(body)
	}
	if err := r.install(); err == nil {
		t.Fatal("a truncated download was accepted")
	}
	r.untouched()
}

func TestADownloadOfTheWrongSizeIsRefusedEvenIfTheBytesArrived(t *testing.T) {
	r := newRig(t, releaseZip(t))
	r.asset.Size += 10
	r.failedWith(r.install(), "cut short")
	r.untouched()
}

func TestAnUpdateBiggerThanTheLimitIsNotDownloaded(t *testing.T) {
	r := newRig(t, releaseZip(t))
	r.inst.limit = 100
	r.failedWith(r.install(), "more than the 100")
	r.untouched()
}

func TestAnAddressOutsideTheProjectsReleasesIsNeverContacted(t *testing.T) {
	r := newRig(t, releaseZip(t))
	for _, url := range []string{"https://evil.example/omni.zip", "http://github.com/Adityuhh11/Omni/releases/download/v1/omni.zip", ""} {
		r.asset.URL = url
		if err := r.install(); err == nil {
			t.Errorf("%q was accepted", url)
		}
	}
	if r.hits.Load() != 0 {
		t.Fatalf("the server was asked %d times", r.hits.Load())
	}
	r.untouched()
}

func TestTheRealPrefixIsThisProjectsReleases(t *testing.T) {
	if NewInstaller(t.TempDir()).prefix != "https://github.com/Adityuhh11/Omni/releases/download/" {
		t.Fatal("the installer would download from somewhere else")
	}
}

func TestAServerThatAnswersAnErrorInstallsNothing(t *testing.T) {
	r := newRig(t, releaseZip(t))
	r.handle = func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "gone", http.StatusNotFound) }
	r.failedWith(r.install(), "404")
	r.untouched()
}

func TestARedirectIsFollowedToTheFile(t *testing.T) {
	body := releaseZip(t)
	r := newRig(t, body)
	r.handle = func(w http.ResponseWriter, req *http.Request) {
		if strings.HasSuffix(req.URL.Path, "/final.zip") {
			w.Write(body)
			return
		}
		http.Redirect(w, req, "/v9.9.9/final.zip", http.StatusFound)
	}
	if err := r.install(); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(r.exe); !bytes.Equal(got, newProgram) {
		t.Fatalf("the program is %q", got)
	}
}

func TestAZipThatDoesNotHoldExactlyOneOmniExeAtItsTopInstallsNothing(t *testing.T) {
	for name, tc := range map[string]struct {
		body []byte
		want string
	}{
		"only a readme":     {zipOf(t, zipEntry{"README.txt", []byte("x")}), "no omni.exe"},
		"nested":            {zipOf(t, zipEntry{"bin/omni.exe", newProgram}), "no omni.exe"},
		"a path out":        {zipOf(t, zipEntry{"../omni.exe", newProgram}), "no omni.exe"},
		"another case":      {zipOf(t, zipEntry{"Omni.EXE", newProgram}), "no omni.exe"},
		"twice":             {zipOf(t, zipEntry{"omni.exe", newProgram}, zipEntry{"omni.exe", newProgram}), "twice"},
		"a directory":       {zipOf(t, zipEntry{"omni.exe/", nil}), "no omni.exe"},
		"not a program":     {zipOf(t, zipEntry{"omni.exe", []byte("#!/bin/sh\necho hi")}), "not a Windows program"},
		"an empty file":     {zipOf(t, zipEntry{"omni.exe", nil}), "not a Windows program"},
		"not even a zip":    {[]byte("this is not a zip file at all"), "zip"},
		"a zip of nothing":  {zipOf(t), "no omni.exe"},
		"program and extra": {zipOf(t, zipEntry{"omni.exe", newProgram}, zipEntry{"../evil.dll", []byte("x")}), ""},
	} {
		r := newRig(t, tc.body)
		parent := filepath.Dir(r.dir)
		before, _ := os.ReadDir(parent)
		err := r.install()
		if tc.want == "" { // the extra entry is ignored: only omni.exe is ever read
			if err != nil {
				t.Errorf("%s: %v", name, err)
			}
			if _, statErr := os.Stat(filepath.Join(parent, "evil.dll")); statErr == nil {
				t.Errorf("%s: an entry's path put a file outside the folder", name)
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: error %v, want one containing %q", name, err, tc.want)
			continue
		}
		r.untouched()
		after, _ := os.ReadDir(parent)
		if len(after) != len(before) {
			t.Errorf("%s: something appeared beside the program's folder", name)
		}
	}
}

func TestAFolderThatCannotBeWrittenIsFoundOutBeforeTheDownload(t *testing.T) {
	r := newRig(t, releaseZip(t))
	gone := filepath.Join(r.dir, "no", "such", "folder", "omni.exe")
	err := r.inst.Install(context.Background(), r.asset, gone)
	if err == nil || !strings.Contains(err.Error(), "cannot write next to omni.exe") {
		t.Fatalf("error %v", err)
	}
	if r.hits.Load() != 0 {
		t.Fatal("the update was downloaded into a folder that cannot be written")
	}
}

func TestIfTheNewProgramCannotTakeItsPlaceTheOldOneIsPutBack(t *testing.T) {
	r := newRig(t, releaseZip(t))
	r.inst.rename = func(from, to string) error {
		if from == r.exe+".new" {
			return errors.New("the virus scanner has it")
		}
		return os.Rename(from, to)
	}
	r.failedWith(r.install(), "the old one is back in place")
	r.untouched()
}

func TestIfTheRunningProgramCannotBeMovedAsideNothingChanges(t *testing.T) {
	r := newRig(t, releaseZip(t))
	r.inst.rename = func(from, to string) error {
		if from == r.exe {
			return errors.New("access is denied")
		}
		return os.Rename(from, to)
	}
	r.failedWith(r.install(), "move the running program aside")
	r.untouched()
}

func TestIfEvenPuttingTheOldOneBackFailsTheErrorSaysWhereItIs(t *testing.T) {
	r := newRig(t, releaseZip(t))
	r.inst.rename = func(from, to string) error {
		if from == r.exe+".new" || (from == r.exe+".old" && to == r.exe) {
			return errors.New("denied")
		}
		return os.Rename(from, to)
	}
	err := r.install()
	if err == nil || !strings.Contains(err.Error(), r.exe+".old") {
		t.Fatalf("error %v, want it to name %s", err, r.exe+".old")
	}
	if got, _ := os.ReadFile(r.exe + ".old"); !bytes.Equal(got, oldProgram) {
		t.Fatal("the old program is not where the error says")
	}
}

func TestARenameThatFailsTwiceAndThenWorksIsRetried(t *testing.T) {
	r := newRig(t, releaseZip(t))
	failures := 0
	r.inst.rename = func(from, to string) error {
		if from == r.exe+".new" && failures < 2 {
			failures++
			return errors.New("busy")
		}
		return os.Rename(from, to)
	}
	if err := r.install(); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(r.exe); !bytes.Equal(got, newProgram) || failures != 2 {
		t.Fatalf("program %q after %d failures", got, failures)
	}
}

func TestLeftoversOfAnEarlierUpdateAreReplaced(t *testing.T) {
	r := newRig(t, releaseZip(t))
	if err := os.WriteFile(r.exe+".old", []byte("an older program"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(r.exe+".new", []byte("half a download"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := r.install(); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(r.exe + ".old"); !bytes.Equal(got, oldProgram) {
		t.Fatalf(".old is %q", got)
	}
	r.onlyThe([]string{"omni.exe", "omni.exe.old"})
}

func TestAReleaseWithNothingToVerifyIsNotInstalled(t *testing.T) {
	r := newRig(t, releaseZip(t))
	r.asset = Asset{}
	r.failedWith(r.install(), "no zip that can be checked")
	r.untouched()
}

// block makes the server hold the download until the test lets go or the request is cancelled.
func (r *rig) block() (release func()) {
	gate := make(chan struct{})
	r.handle = func(w http.ResponseWriter, req *http.Request) {
		select {
		case <-gate:
		case <-req.Context().Done():
		}
	}
	return func() { close(gate) }
}

func waitFor(t *testing.T, what string, ok func() bool) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(5 * time.Millisecond) {
		if ok() {
			return
		}
	}
	t.Fatalf("timed out waiting for %s", what)
}

func TestCancellingADownloadStopsItAndLeavesTheProgramAlone(t *testing.T) {
	r := newRig(t, releaseZip(t))
	r.block()
	finished := make(chan error, 1)
	if err := r.inst.Start(r.asset, r.exe, func(err error) { finished <- err }); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the download to start", func() bool { return r.hits.Load() == 1 })
	r.inst.Cancel()
	select {
	case err := <-finished:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("finished with %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the install did not stop")
	}
	if p := r.inst.Progress(); p.Stage != StageFailed || p.Error != "The update was cancelled." {
		t.Fatalf("progress %+v", p)
	}
	r.untouched()
}

func TestASecondInstallIsRefusedWhileOneRunsAndAllowedAfterwards(t *testing.T) {
	r := newRig(t, releaseZip(t))
	release := r.block()
	finished := make(chan error, 1)
	if err := r.inst.Start(r.asset, r.exe, func(err error) { finished <- err }); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the download to start", func() bool { return r.hits.Load() == 1 })
	if err := r.inst.Start(r.asset, r.exe, func(error) { t.Error("the refused install ran") }); err == nil {
		t.Fatal("a second install started")
	}
	r.inst.Cancel()
	release()
	<-finished
	body := releaseZip(t)
	r.handle = func(w http.ResponseWriter, _ *http.Request) { w.Write(body) }
	if err := r.inst.Start(r.asset, r.exe, func(error) { finished <- nil }); err != nil {
		t.Fatalf("a new install after the first ended: %v", err)
	}
	<-finished
}

func TestStartTellsDoneOnceWithNilOnSuccess(t *testing.T) {
	r := newRig(t, releaseZip(t))
	finished := make(chan error, 2)
	if err := r.inst.Start(r.asset, r.exe, func(err error) { finished <- err }); err != nil {
		t.Fatal(err)
	}
	if err := <-finished; err != nil {
		t.Fatal(err)
	}
	select {
	case <-finished:
		t.Fatal("done was called twice")
	case <-time.After(100 * time.Millisecond):
	}
	if p := r.inst.Progress(); p.Stage != StageDone {
		t.Fatalf("progress %+v", p)
	}
}

func TestAnIdleInstallerSaysSo(t *testing.T) {
	if p := NewInstaller(t.TempDir()).Progress(); p.Stage != StageIdle || p.Error != "" {
		t.Fatalf("progress %+v", p)
	}
}

func TestThePageIsGivenASentenceNotAnErrorMessage(t *testing.T) {
	for in, want := range map[string]string{
		"the download does not match the checksum":        "The download does not match the checksum.",
		"cannot write next to omni.exe: access is denied.": "Cannot write next to omni.exe: access is denied.",
		"The update was cancelled.":                       "The update was cancelled.",
		"édition":                                         "Édition.",
		"":                                                "",
	} {
		if got := sentence(in); got != want {
			t.Errorf("sentence(%q) = %q, want %q", in, got, want)
		}
	}
	r := newRig(t, releaseZip(t))
	r.asset.SHA256 = sumOf([]byte("other"))
	_ = r.install()
	if got := r.inst.Progress().Error; !strings.HasPrefix(got, "The download does not match") || !strings.HasSuffix(got, ".") {
		t.Errorf("the page would show %q", got)
	}
}
