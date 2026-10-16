// Package rank decides how well a typed query matches a title and how much a thing's past use should
// lift it. Every instant provider (apps, commands, windows, tags) ranks with it, so they all feel the
// same: type a few letters of a name, or its initials, and the thing you use most comes first.
package rank

import (
	"strings"
	"unicode"
)

// Result is how well a query matches a title.
type Result struct {
	// Score is 0 for no match, otherwise between 0 and 1: higher is a better match.
	Score float64
	// Strong marks a match on the start of the title, the start of its words, or its initials. Weaker
	// matches (letters spread through a name) are real but the bar shows them only when asked.
	Strong bool
}

// Match scores query against title, ignoring case. The tiers, best first: the title starts with the
// query; each word of the query starts a different word of the title (`vis code`); the query is the
// initials of successive words, each optionally continued (`vsc`, `vscode`); and finally the letters of
// the query appear in order starting at a word, which is a weak match.
func Match(query, title string) Result {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return Result{}
	}
	lower := strings.ToLower(title)
	squeezed := strings.ReplaceAll(q, " ", "")
	// Between 0 and 1: how much of the title the query covers, so the shorter name wins a tie.
	cover := min(1, float64(len([]rune(squeezed)))/float64(max(1, len([]rune(lower)))))

	if lower == q {
		return Result{Score: 1, Strong: true}
	}
	if strings.HasPrefix(lower, q) {
		return Result{Score: 0.85 + 0.1*cover, Strong: true}
	}
	words := wordsOf(title)
	if wordPrefixes(strings.Fields(q), words) {
		return Result{Score: 0.7 + 0.1*cover, Strong: true}
	}
	if initials(squeezed, words) {
		return Result{Score: 0.55 + 0.1*cover, Strong: true}
	}
	if score := scattered(squeezed, []rune(lower), words); score > 0 {
		return Result{Score: score}
	}
	return Result{}
}

// word is a lower-case word of a title and where it starts, in runes.
type word struct {
	text  []rune
	start int
}

// wordsOf splits a title at anything that is not a letter or digit and inside camel case, so
// `PowerPoint` is Power and Point and `7-Zip` is 7 and Zip.
func wordsOf(title string) []word {
	var words []word
	runes := []rune(title)
	begin := -1
	flush := func(end int) {
		if begin >= 0 {
			words = append(words, word{text: []rune(strings.ToLower(string(runes[begin:end]))), start: begin})
			begin = -1
		}
	}
	for i, r := range runes {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			flush(i)
			continue
		}
		if begin >= 0 && unicode.IsUpper(r) && unicode.IsLower(runes[i-1]) {
			flush(i)
		}
		if begin < 0 {
			begin = i
		}
	}
	flush(len(runes))
	return words
}

// wordPrefixes reports whether every token is the start of a different word of the title.
func wordPrefixes(tokens []string, words []word) bool {
	if len(tokens) == 0 {
		return false
	}
	used := make([]bool, len(words))
	for _, token := range tokens {
		found := false
		for i, w := range words {
			if !used[i] && strings.HasPrefix(string(w.text), token) {
				used[i], found = true, true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// initials reports whether q is made of consecutive pieces, each the start of a later word than the
// one before: `vsc` is V, S and C of Visual Studio Code, and `vscode` is V, S and the whole of Code.
func initials(q string, words []word) bool {
	var walk func(rest []rune, from int) bool
	walk = func(rest []rune, from int) bool {
		if len(rest) == 0 {
			return true
		}
		for i := from; i < len(words); i++ {
			text := words[i].text
			for n := min(len(rest), len(text)); n >= 1; n-- {
				if string(text[:n]) == string(rest[:n]) && walk(rest[n:], i+1) {
					return true
				}
			}
		}
		return false
	}
	return walk([]rune(q), 0)
}

// scattered finds the letters of q in order, the first at the start of a word, taking each next
// letter as soon as it appears. The score falls as the letters spread out.
//
// ponytail: greedy, not the best alignment; a title with a better later alignment scores a little low.
// Upgrade to a small dynamic program if the owner's real queries show a wrong order.
func scattered(q string, title []rune, words []word) float64 {
	letters := []rune(q)
	if len(letters) < 2 {
		return 0
	}
	for _, w := range words {
		if w.text[0] != letters[0] {
			continue
		}
		at, matched := w.start, 1
		for _, l := range letters[1:] {
			next := -1
			for j := at + 1; j < len(title); j++ {
				if title[j] == l {
					next = j
					break
				}
			}
			if next < 0 {
				break
			}
			at, matched = next, matched+1
		}
		if matched == len(letters) {
			return 0.4 * float64(len(letters)) / float64(at-w.start+1)
		}
	}
	return 0
}
