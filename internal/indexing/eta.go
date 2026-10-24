package indexing

import (
	"fmt"
	"time"
)

// An estimate needs a few files done and a few seconds worked, or the first slow file would promise hours.
const (
	minFilesForEstimate = 5
	minWorkForEstimate  = 5 * time.Second
)

// remaining estimates how long a job has left from the time it has actually worked (a pause does not count) and how
// many of its files are done. Files differ a lot in cost, so it is rounded to the minute, and 0 means "no estimate yet".
func remaining(worked time.Duration, done, total int) time.Duration {
	if done < minFilesForEstimate || worked < minWorkForEstimate || done >= total {
		return 0
	}
	left := time.Duration(float64(worked) / float64(done) * float64(total-done))
	return max(left.Round(time.Minute), time.Minute)
}

// LeftText is the end of a status sentence ("about 10 min left"), or "" when there is no estimate.
func LeftText(left time.Duration) string {
	switch {
	case left <= 0:
		return ""
	case left <= time.Minute:
		return ", less than a minute left"
	case left < 90*time.Minute:
		return fmt.Sprintf(", about %d min left", int(left.Minutes()))
	default:
		return fmt.Sprintf(", about %d h left", int(left.Round(time.Hour).Hours()))
	}
}
