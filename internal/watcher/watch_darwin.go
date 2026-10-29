package watcher

import (
	"context"
	"log"
	"time"
)

// Watch calls changed(root) for each folder in roots whenever something relevant under it has changed and then been quiet for quiet
// (or after longest, if it never goes quiet). It blocks until ctx ends.
//
// It watches nothing yet: FSEvents arrives in milestone M3. Until then a change reaches the index at the next rescan
// ("Rescan now" in the tray, and each start), and a program installed or removed reaches the app list at its daily refresh.
func Watch(ctx context.Context, roots []string, quiet, longest time.Duration, changed func(root string)) {
	log.Printf("watcher: watching %d folders is not available on macOS yet; changes are found at the next rescan", len(roots))
	<-ctx.Done()
}
