package settingsui

import (
	"context"
	"log"
	"time"

	"agi/internal/config"
	"agi/internal/update"
)

// The About page: the update check, and installing a newer release.

func (s *Service) updateFound() string {
	rel, _ := s.o.Updates.Found()
	return rel.Version
}

func (s *Service) updateInstallable() bool {
	rel, ok := s.o.Updates.Found()
	return ok && rel.Asset.URL != ""
}

// SetCheckUpdates turns the daily update check on or off.
func (s *Service) SetCheckUpdates(on bool) string {
	return reason(s.o.Store.Update(func(c *config.Config) { c.CheckUpdates = on }))
}

// UpdateView is what a check found: the newest release, whether it is newer than this build, or why
// the check failed.
type UpdateView struct {
	Latest    string
	Available bool
	// Installable says the newer release has a zip that can be checked, so it can be installed from the page.
	Installable bool
	Error       string
}

// CheckUpdate asks GitHub for the newest release now.
func (s *Service) CheckUpdate() UpdateView {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	rel, newer, err := s.o.Updates.Check(ctx)
	if err != nil {
		return UpdateView{Error: err.Error()}
	}
	return UpdateView{Latest: rel.Version, Available: newer, Installable: newer && rel.Asset.URL != ""}
}

// UpdateProgress is where the install is, for the page: the stage ("idle", "downloading", "installing", "done" or "failed"),
// the bytes of the download, and the reason for a failure (or for a restart that did not happen).
type UpdateProgress struct {
	Stage       string
	Done, Total int64
	Error       string
}

// UpdateProgress says where the install of the newer release is.
func (s *Service) UpdateProgress() UpdateProgress {
	p := s.o.Installer.Progress()
	out := UpdateProgress{Stage: string(p.Stage), Done: p.Done, Total: p.Total, Error: p.Error}
	s.mu.Lock()
	defer s.mu.Unlock()
	if p.Stage == update.StageDone && s.restartErr != "" {
		out.Error = "The update is installed, but the program could not restart itself (" + s.restartErr + "). Quit it from the tray icon and start it again."
	}
	return out
}

// StartUpdate installs the newer release the last check found, in the background, and restarts the launcher when it is
// in place. It returns why it could not start, or "".
func (s *Service) StartUpdate() string {
	rel, ok := s.o.Updates.Found()
	if !ok {
		return "there is no newer release to install"
	}
	if rel.Asset.URL == "" {
		return "this release has no zip that can be checked, so it cannot be installed from here: open the release page"
	}
	s.mu.Lock()
	s.restartErr = ""
	s.mu.Unlock()
	return reason(s.o.Installer.Start(rel.Asset, s.o.ExePath, func(err error) {
		if err != nil {
			log.Printf("update to %s failed: %v", rel.Version, err)
			return
		}
		log.Printf("update to %s installed; restarting", rel.Version)
		if err := s.o.Restart(); err != nil {
			log.Printf("update to %s installed but the restart failed: %v", rel.Version, err)
			s.mu.Lock()
			s.restartErr = err.Error()
			s.mu.Unlock()
		}
	}))
}

// CancelUpdate stops an install that is still downloading.
func (s *Service) CancelUpdate() { s.o.Installer.Cancel() }

// OpenUpdate opens the page of the newer release the last check found. The address comes from that
// check, never from the window.
func (s *Service) OpenUpdate() string {
	rel, ok := s.o.Updates.Found()
	if !ok {
		return "there is no newer release to open"
	}
	return reason(s.o.OpenURL(rel.URL))
}
