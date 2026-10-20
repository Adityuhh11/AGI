package indexing

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"agi/internal/config"
	"agi/internal/sidecar"
)

// cycle runs one sync of the roots in only (every root when only is nil) and reports whether it
// succeeded. A full cycle also covers whatever the watcher had reported so far.
func (m *Manager) cycle(ctx context.Context, only map[string]bool) bool {
	if only == nil {
		m.takeDirty()
	}
	if !m.ready() {
		m.set(Status{State: Waiting})
	}
	for !m.ready() {
		select {
		case <-ctx.Done():
			return true
		case <-time.After(m.poll):
		}
	}
	m.applyPace(ctx) // an engine that has just started knows nothing of the speed or the pause chosen
	problems, err := m.syncAll(ctx, only)
	if ctx.Err() != nil {
		return true
	}
	if err != nil {
		problems = append(problems, err.Error())
	}
	if len(problems) > 0 {
		m.set(Status{State: Failed, Message: strings.Join(problems, "; ")})
		log.Print("indexing: ", problems)
		return false
	}
	return true
}

// wasDeleted reports whether root is gone while the folder that held it is still there: the user
// deleted it. A root whose parent is missing too (an unplugged drive) is not a deletion, so its
// index is kept for when the drive is back.
func wasDeleted(root string) bool {
	if _, err := os.Stat(root); !errors.Is(err, fs.ErrNotExist) {
		return false
	}
	_, err := os.Stat(filepath.Dir(root))
	return err == nil
}

// syncAll forgets sources that are no longer configured (or whose folder was deleted) and syncs
// every other root, or only those in only when it is not nil. A root that fails does not stop the
// others; its problem is returned.
func (m *Manager) syncAll(ctx context.Context, only map[string]bool) (problems []string, err error) {
	var roots []string
	configured, exclude := m.folders()
	ids := make(map[string]bool, len(configured))
	for _, root := range configured {
		if m.deleted(root) {
			log.Printf("indexing: %s no longer exists, so its index entries are dropped", root)
			continue
		}
		roots = append(roots, root)
		ids[config.SourceID(root)] = true
	}
	sources, err := m.engine.Sources(ctx)
	if err != nil {
		return nil, fmt.Errorf("list indexed folders: %w", err)
	}
	for _, source := range sources {
		if ids[source.ID] {
			continue
		}
		if _, err := m.engine.DeleteSource(ctx, source.ID); err != nil {
			return nil, fmt.Errorf("forget %s: %w", source.Root, err)
		}
	}
	for _, root := range roots {
		if only != nil && !only[root] {
			continue
		}
		m.setFolder(FolderStatus{Root: root, State: Waiting})
	}
	for _, root := range roots {
		if only != nil && !only[root] {
			continue
		}
		if err := m.syncRoot(ctx, root, exclude); err != nil {
			problems = append(problems, filepath.Base(root)+": "+err.Error())
			m.setFolder(FolderStatus{Root: root, State: Failed, Message: err.Error()})
		}
	}
	// Totals are read after the last job, so they count what the index holds now.
	if sources, err = m.engine.Sources(ctx); err != nil {
		return nil, fmt.Errorf("list indexed folders: %w", err)
	}
	for _, root := range roots {
		if only != nil && !only[root] || m.failed(root) {
			continue
		}
		for _, source := range sources {
			if source.ID == config.SourceID(root) {
				m.setFolder(FolderStatus{Root: root, State: UpToDate, Files: source.Files, Chunks: source.Chunks})
			}
		}
	}
	if len(problems) > 0 {
		return problems, nil
	}
	total := Status{State: UpToDate}
	for _, source := range sources {
		if ids[source.ID] {
			total.Files += source.Files
			total.Chunks += source.Chunks
		}
	}
	m.set(total)
	return nil, nil
}

func (m *Manager) syncRoot(ctx context.Context, root string, exclude []string) error {
	id := config.SourceID(root)
	for {
		jobID, err := m.engine.AddSource(ctx, root, id, config.ExcludedUnder(root, exclude))
		var busy *sidecar.BusyError
		if err == nil {
			return m.follow(ctx, root, jobID)
		}
		if !errors.As(err, &busy) || busy.JobID == "" {
			return err
		}
		// Another job is running (only after a launcher restart that the engine outlived): let it
		// finish, then submit again.
		if err := m.follow(ctx, root, busy.JobID); err != nil {
			return err
		}
	}
}

// follow reports the job's progress until it ends.
func (m *Manager) follow(ctx context.Context, root, jobID string) error {
	for {
		job, err := m.engine.Job(ctx, jobID)
		if err != nil {
			return err
		}
		switch job.State {
		case sidecar.JobDone:
			if job.Report != nil && len(job.Report.Errors) > 0 {
				log.Printf("indexing %s: %d files could not be read, e.g. %s: %s", root, len(job.Report.Errors),
					job.Report.Errors[0].URI, job.Report.Errors[0].Message)
			}
			return nil
		case sidecar.JobFailed:
			if job.Error != nil {
				return errors.New(*job.Error)
			}
			return errors.New("the job failed")
		}
		m.setFolder(FolderStatus{Root: root, State: Indexing, FilesDone: job.FilesDone, FilesTotal: job.FilesTotal})
		m.set(Status{State: Indexing, Root: root, FilesDone: job.FilesDone, FilesTotal: job.FilesTotal})
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(m.poll):
		}
	}
}
