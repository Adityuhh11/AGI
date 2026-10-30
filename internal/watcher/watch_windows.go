//go:build windows

package watcher

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	notifyMask = windows.FILE_NOTIFY_CHANGE_FILE_NAME | windows.FILE_NOTIFY_CHANGE_DIR_NAME |
		windows.FILE_NOTIFY_CHANGE_SIZE | windows.FILE_NOTIFY_CHANGE_LAST_WRITE
	// bufferBytes holds the events between two reads; if more arrive the call returns 0 bytes and
	// the whole folder counts as changed.
	bufferBytes = 64 << 10
)

// Watch calls changed(root) for each folder in roots whenever something relevant under it has
// changed and then been quiet for quiet (or after longest, if it never goes quiet). It blocks until
// ctx ends. A folder that cannot be watched is logged and tried again later; the others still work.
func Watch(ctx context.Context, roots []string, quiet, longest time.Duration, changed func(root string)) {
	fire := newDebouncer(quiet, longest, changed)
	defer fire.stop()
	var running sync.WaitGroup
	for _, root := range roots {
		running.Add(1)
		go func() {
			defer running.Done()
			keepWatching(ctx, root, fire)
		}()
	}
	running.Wait()
}

// retryAfter is how long a folder that cannot be watched (a drive that was unplugged, a share that dropped, a folder not made yet)
// waits before it is tried again. A variable so a test need not wait for it.
var retryAfter = 30 * time.Second

// keepWatching watches root until ctx ends, and when the watch fails, or the folder cannot be opened, tries again every
// retryAfter. When it works again the folder is reported as changed, since what happened while it was not watched is unknown.
func keepWatching(ctx context.Context, root string, fire *debouncer) {
	for failed := false; ctx.Err() == nil; failed = true {
		handle, err := openDirectory(root)
		if err == nil {
			if failed {
				fire.poke(root)
			}
			err = read(ctx, handle, root, fire)
		}
		if ctx.Err() != nil {
			return
		}
		if !failed { // once, not every retry: a folder on a drive that stays away would fill the log
			log.Printf("watcher: cannot watch %s, trying again every %s: %v", root, retryAfter, err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(retryAfter):
		}
	}
}

func openDirectory(path string) (windows.Handle, error) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	return windows.CreateFile(name, windows.FILE_LIST_DIRECTORY,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil,
		windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
}

// read waits for changes under root until ctx ends. The call blocks, so a second goroutine cancels
// it when ctx ends; the handle is closed only after read has stopped using it.
func read(ctx context.Context, handle windows.Handle, root string, fire *debouncer) error {
	finished, canceller := make(chan struct{}), make(chan struct{})
	defer func() {
		close(finished)
		<-canceller // so the handle is not closed while a cancel is still being sent to it
		_ = windows.CloseHandle(handle)
	}()
	go func() {
		defer close(canceller)
		select {
		case <-ctx.Done():
		case <-finished:
			return
		}
		// A cancel that arrives before the read below has begun cancels nothing, so it is repeated until read has stopped.
		for {
			_ = windows.CancelIoEx(handle, nil)
			select {
			case <-finished:
				return
			case <-time.After(20 * time.Millisecond):
			}
		}
	}()

	buf := make([]byte, bufferBytes)
	for {
		var n uint32
		err := windows.ReadDirectoryChanges(handle, &buf[0], uint32(len(buf)), true, notifyMask, &n, nil, 0)
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, windows.ERROR_OPERATION_ABORTED) {
				return nil
			}
			return fmt.Errorf("read changes: %w", err)
		}
		if n == 0 { // the buffer overflowed: anything may have changed
			fire.poke(root)
			continue
		}
		for offset := uint32(0); ; {
			info := (*windows.FileNotifyInformation)(unsafe.Pointer(&buf[offset]))
			name := windows.UTF16ToString(unsafe.Slice(&info.FileName, info.FileNameLength/2))
			if relevant(name) {
				fire.poke(root)
			}
			if info.NextEntryOffset == 0 {
				break
			}
			offset += info.NextEntryOffset
		}
	}
}
