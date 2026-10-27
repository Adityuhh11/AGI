package update

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"
)

// Stage is where an install is.
type Stage string

const (
	StageIdle        Stage = "idle"
	StageDownloading Stage = "downloading"
	StageInstalling  Stage = "installing"
	StageDone        Stage = "done"
	StageFailed      Stage = "failed"
)

// Progress is what the settings page shows of an install: the stage, the bytes of the download, and why it failed.
type Progress struct {
	Stage       Stage
	Done, Total int64
	Error       string
}

// installTimeout is the longest an install may take: the zip is 5 MB.
const installTimeout = 15 * time.Minute

// Installer downloads a release, checks it and puts it in place of the running program. Only one runs at a time.
type Installer struct {
	workDir string // where the download is kept while it is checked
	prefix  string // the addresses a download may come from
	limit   int64  // the most that is downloaded or unpacked
	client  *http.Client

	// What the tests replace: the rename, the pause between tries and how many tries.
	rename   func(oldPath, newPath string) error
	sleep    func(time.Duration)
	attempts int
	pause    time.Duration

	mu       sync.Mutex
	progress Progress
	running  bool
	cancel   context.CancelFunc
}

// NewInstaller returns an Installer that keeps its download in workDir.
func NewInstaller(workDir string) *Installer {
	i := &Installer{
		workDir: workDir, prefix: DownloadPrefix, limit: maxZip,
		rename: os.Rename, sleep: time.Sleep, attempts: swapAttempts, pause: swapPause,
		progress: Progress{Stage: StageIdle},
	}
	i.client = &http.Client{
		Transport: &http.Transport{Proxy: http.ProxyFromEnvironment, ResponseHeaderTimeout: 30 * time.Second},
		// Redirects are followed (GitHub sends the file from another host) but never to a plain-http address.
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return errors.New("too many redirects")
			}
			if req.URL.Scheme != via[0].URL.Scheme {
				return fmt.Errorf("a redirect from %s to %s is refused", via[0].URL.Scheme, req.URL.Scheme)
			}
			return nil
		},
	}
	return i
}

// sentence makes an error message, which starts in lower case and has no full stop, into a sentence for the page.
func sentence(msg string) string {
	if msg == "" {
		return msg
	}
	r, size := utf8.DecodeRuneInString(msg)
	msg = string(unicode.ToUpper(r)) + msg[size:]
	if !strings.HasSuffix(msg, ".") {
		msg += "."
	}
	return msg
}

// Progress says where the install is.
func (i *Installer) Progress() Progress {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.progress
}

func (i *Installer) set(p Progress) {
	i.mu.Lock()
	i.progress = p
	i.mu.Unlock()
}

// Cancel stops a running install: before the swap nothing on disk has changed.
func (i *Installer) Cancel() {
	i.mu.Lock()
	cancel := i.cancel
	i.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// Start installs asset over the program at exePath in the background and tells done how it ended. A second
// install is refused while one runs.
func (i *Installer) Start(asset Asset, exePath string, done func(error)) error {
	i.mu.Lock()
	if i.running {
		i.mu.Unlock()
		return errors.New("an update is already being installed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), installTimeout)
	i.running, i.cancel = true, cancel
	i.mu.Unlock()
	go func() {
		err := i.Install(ctx, asset, exePath)
		cancel()
		i.mu.Lock()
		i.running, i.cancel = false, nil
		i.mu.Unlock()
		done(err)
	}()
	return nil
}

// Install does the work of Start in the calling goroutine: check the folder can be written, download, verify,
// unpack omni.exe next to the program, swap. Until the swap nothing that exists is changed.
func (i *Installer) Install(ctx context.Context, asset Asset, exePath string) (err error) {
	defer func() {
		if err != nil {
			msg := err.Error()
			if errors.Is(err, context.Canceled) {
				msg = "The update was cancelled."
			}
			i.set(Progress{Stage: StageFailed, Error: sentence(msg)})
		}
	}()
	if asset.URL == "" || asset.SHA256 == "" {
		return errors.New("this release has no zip that can be checked, so it is not installed from here")
	}
	i.set(Progress{Stage: StageDownloading, Total: asset.Size})

	// Find out now, not after the download, that the folder cannot be written.
	fresh := exePath + ".new"
	_ = os.Remove(fresh)
	probe, err := os.OpenFile(fresh, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o755)
	if err != nil {
		return fmt.Errorf("cannot write next to %s: %w", filepath.Base(exePath), err)
	}
	_ = probe.Close()
	_ = os.Remove(fresh)

	_ = os.RemoveAll(i.workDir)
	if err := os.MkdirAll(i.workDir, 0o755); err != nil {
		return fmt.Errorf("create the download folder: %w", err)
	}
	defer os.RemoveAll(i.workDir)

	zipPath, err := i.download(ctx, asset, i.workDir, func(done, total int64) {
		i.set(Progress{Stage: StageDownloading, Done: done, Total: total})
	})
	if err != nil {
		return err
	}
	i.set(Progress{Stage: StageInstalling})
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := i.extract(zipPath, fresh); err != nil {
		return err
	}
	if err := i.swap(exePath, fresh); err != nil {
		_ = os.Remove(fresh)
		return err
	}
	i.set(Progress{Stage: StageDone})
	return nil
}
