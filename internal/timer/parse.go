package timer

import (
	"strconv"
	"strings"
	"time"
	"unicode"
)

const (
	minDuration = time.Second
	maxDuration = 24 * time.Hour
)

// ParseDuration reads the time at the start of what was typed after "timer": "25" (minutes), "90s", "1h30", "1.5h",
// "10m tea", "1 h 30 min". The words after the time are the label. ok is false when the text does not start with a time or
// the time is out of range (under a second, over a day).
func ParseDuration(text string) (d time.Duration, label string, ok bool) {
	rest := strings.TrimSpace(text)
	var total float64 // seconds
	seen := map[string]bool{}
	last := "" // the unit of the previous number, which says what a bare number after it means
	for {
		number, after, found := leadingNumber(rest)
		if !found {
			break
		}
		unit, afterUnit := leadingUnit(strings.TrimLeft(after, " "))
		if unit == "" {
			// A bare number is minutes on its own, minutes after hours ("1h30") and seconds after minutes ("1m30").
			switch last {
			case "", "h":
				unit = "m"
			case "m":
				unit = "s"
			default:
				return 0, "", false
			}
			if strings.HasPrefix(after, ":") { // 1:30 is not a form we read; guessing what it means would be worse
				return 0, "", false
			}
			afterUnit = after
		}
		if seen[unit] || last == "s" || (last == "m" && unit == "h") {
			break // a unit twice, or one out of order, is where the label starts
		}
		seen[unit] = true
		last = unit
		total += number * map[string]float64{"h": 3600, "m": 60, "s": 1}[unit]
		rest = strings.TrimSpace(afterUnit)
	}
	if len(seen) == 0 {
		return 0, "", false
	}
	d = time.Duration(total * float64(time.Second))
	if d < minDuration || d > maxDuration {
		return 0, "", false
	}
	return d, strings.TrimSpace(rest), true
}

// leadingNumber reads "12" or "1.5" at the start of s.
func leadingNumber(s string) (number float64, rest string, ok bool) {
	end := 0
	dot := false
	for end < len(s) && (s[end] >= '0' && s[end] <= '9' || (s[end] == '.' && !dot && end > 0)) {
		dot = dot || s[end] == '.'
		end++
	}
	if end == 0 {
		return 0, s, false
	}
	n, err := strconv.ParseFloat(strings.TrimRight(s[:end], "."), 64)
	if err != nil {
		return 0, s, false
	}
	return n, s[end:], true
}

var units = map[string]string{
	"h": "h", "hr": "h", "hrs": "h", "hour": "h", "hours": "h",
	"m": "m", "min": "m", "mins": "m", "minute": "m", "minutes": "m",
	"s": "s", "sec": "s", "secs": "s", "second": "s", "seconds": "s",
}

// leadingUnit reads a unit word at the start of s ("min", "h").
func leadingUnit(s string) (unit, rest string) {
	end := 0
	for end < len(s) && unicode.IsLetter(rune(s[end])) {
		end++
	}
	if u, ok := units[strings.ToLower(s[:end])]; ok {
		return u, s[end:]
	}
	return "", s
}
