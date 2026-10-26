// Package calc answers arithmetic and unit conversions typed in the bar: "12*7+3", "15% of 80", "5 km in miles".
// It evaluates by its own small parser (no eval, no dependency) and only answers when the whole text is an expression, so an
// ordinary search is never mistaken for one.
package calc

import (
	"errors"
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// Result is an answer.
type Result struct {
	// Title is the answer as shown: "84", or "5 km = 3.10686 mi" for a conversion.
	Title string
	// Copy is what Enter copies: the number alone.
	Copy string
}

// Answer evaluates text. ok is false when the text is not a calculation (or cannot be answered: a division by zero).
// A bare number is not a calculation; neither is a word.
func Answer(text string) (Result, bool) {
	text = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(text), "="))
	if text == "" || len(text) > maxLength {
		return Result{}, false
	}
	if looksLikeADate.MatchString(text) {
		return Result{}, false
	}
	if r, ok := convert(text); ok {
		return r, true
	}
	value, interesting, err := evaluate(text)
	if err != nil || !interesting || math.IsNaN(value) || math.IsInf(value, 0) {
		return Result{}, false
	}
	return Result{Title: pretty(value), Copy: plain(value)}, true
}

// looksLikeADate is "2024-01-05", "10-12", "1/2/2026": numbers joined by dashes, or by two slashes, with no spaces. Typed in
// the bar these are far more likely a date or a range being looked for than a subtraction (a subtraction is "10 - 12").
var looksLikeADate = regexp.MustCompile(`^\d+(-\d+)+$|^\d+/\d+/\d+$`)

// plain is a number as it is copied: no separators, at most 10 significant digits, no trailing zeros.
func plain(v float64) string {
	if v == 0 {
		return "0" // not "-0"
	}
	s := strconv.FormatFloat(v, 'g', 10, 64)
	if strings.ContainsAny(s, "e") {
		return strconv.FormatFloat(v, 'f', -1, 64)
	}
	return s
}

// pretty is a number as it is shown: thousands separated in the whole part.
func pretty(v float64) string {
	s := plain(v)
	whole, frac, hasFrac := strings.Cut(s, ".")
	sign := ""
	if strings.HasPrefix(whole, "-") {
		sign, whole = "-", whole[1:]
	}
	if len(whole) > 4 {
		var b strings.Builder
		for i, r := range whole {
			if i > 0 && (len(whole)-i)%3 == 0 {
				b.WriteByte(',')
			}
			b.WriteRune(r)
		}
		whole = b.String()
	}
	if hasFrac {
		return sign + whole + "." + frac
	}
	return sign + whole
}

// --- the expression parser: expr = term {(+|-) term}; term = unary {(*|/) unary}; unary = (-|+) unary | power;
// power = postfix [^ unary]; postfix = primary [%]; primary = number | name | name( expr ) | ( expr ).
// So -2^2 is -(2^2), 2^-2 is allowed and 2^3^2 is 2^(3^2). "a% of b" is a percentage of b.

type parser struct {
	in    []rune
	at    int
	calls bool // a function, a constant or an operator was used: the text is not just a number
	depth int  // how deeply brackets, signs and powers are nested
}

// maxDepth and maxLength keep a pasted wall of brackets or minus signs from being parsed all the way down.
const (
	maxDepth  = 64
	maxLength = 200
)

// enter counts one level of nesting; the caller must call leave when it is done.
func (p *parser) enter() error {
	p.depth++
	if p.depth > maxDepth {
		return errSyntax
	}
	return nil
}

func (p *parser) leave() { p.depth-- }

var errSyntax = errors.New("not an expression")

// thousands is a number written with commas as thousands separators: 1,000 and 12,345,678.9, not 1,5 or 1,00.
var thousands = regexp.MustCompile(`^\d{1,3}(,\d{3})+(\.\d+)?$`)

func evaluate(text string) (value float64, interesting bool, err error) {
	p := &parser{in: []rune(text)}
	value, _, err = p.expr()
	if err != nil {
		return 0, false, err
	}
	p.space()
	if p.at != len(p.in) {
		return 0, false, errSyntax
	}
	return value, p.calls, nil
}

func (p *parser) space() {
	for p.at < len(p.in) && unicode.IsSpace(p.in[p.at]) {
		p.at++
	}
}

func (p *parser) peek() rune {
	p.space()
	if p.at < len(p.in) {
		return p.in[p.at]
	}
	return 0
}

// expr returns the value and whether it ended in a percent literal (so "100 + 10%" means 110).
func (p *parser) expr() (float64, bool, error) {
	left, percent, err := p.term()
	if err != nil {
		return 0, false, err
	}
	for {
		op := p.peek()
		if op != '+' && op != '-' {
			return left, percent, nil
		}
		p.at++
		p.calls = true
		right, rightPercent, err := p.term()
		if err != nil {
			return 0, false, err
		}
		if rightPercent && !percent { // 100 + 10% is 100 plus ten percent of it; 10% + 5% is just 15%
			right = left * right
		}
		if op == '+' {
			left += right
		} else {
			left -= right
		}
		percent = false
	}
}

func (p *parser) term() (float64, bool, error) {
	left, percent, err := p.unary()
	if err != nil {
		return 0, false, err
	}
	for {
		op := p.peek()
		switch {
		case op == '×' || op == '*' || op == '/' || op == '÷':
		case op == 'x' && p.at+1 < len(p.in) && unicode.IsSpace(p.in[p.at+1]):
			// "3 x 4": an x between spaces; an x inside a word is never an operator
		default:
			return left, percent, nil
		}
		p.at++
		p.calls = true
		right, _, err := p.unary()
		if err != nil {
			return 0, false, err
		}
		if op == '/' || op == '÷' {
			if right == 0 {
				return 0, false, errors.New("division by zero")
			}
			left /= right
		} else {
			left *= right
		}
		percent = false
	}
}

func (p *parser) unary() (float64, bool, error) {
	if err := p.enter(); err != nil {
		return 0, false, err
	}
	defer p.leave()
	switch p.peek() {
	case '-':
		p.at++
		v, percent, err := p.unary()
		return -v, percent, err
	case '+':
		p.at++
		return p.unary()
	}
	return p.power()
}

func (p *parser) power() (float64, bool, error) {
	base, percent, err := p.postfix()
	if err != nil {
		return 0, false, err
	}
	if p.peek() == '^' {
		p.at++
		p.calls = true
		exp, _, err := p.unary() // right to left, and a sign is allowed: 2^3^2 is 2^9, 2^-2 is a quarter
		if err != nil {
			return 0, false, err
		}
		return math.Pow(base, exp), false, nil
	}
	return base, percent, nil
}

func (p *parser) postfix() (float64, bool, error) {
	v, err := p.primary()
	if err != nil {
		return 0, false, err
	}
	if p.peek() == '%' {
		p.at++
		p.calls = true
		// "15% of 80": the percent is a fraction of what follows "of"
		save := p.at
		p.space()
		if p.word() == "of" {
			rest, _, err := p.unary()
			if err != nil {
				return 0, false, err
			}
			return v / 100 * rest, false, nil
		}
		p.at = save
		return v / 100, true, nil
	}
	return v, false, nil
}

// word reads letters at the cursor (and moves past them); "" if there are none.
func (p *parser) word() string {
	start := p.at
	for p.at < len(p.in) && unicode.IsLetter(p.in[p.at]) {
		p.at++
	}
	return strings.ToLower(string(p.in[start:p.at]))
}

var functions = map[string]func(float64) float64{
	"sqrt": math.Sqrt, "abs": math.Abs, "ln": math.Log, "log": math.Log10, "exp": math.Exp,
	"sin":   func(x float64) float64 { return math.Sin(x * math.Pi / 180) },
	"cos":   func(x float64) float64 { return math.Cos(x * math.Pi / 180) },
	"tan":   func(x float64) float64 { return math.Tan(x * math.Pi / 180) },
	"round": math.Round, "floor": math.Floor, "ceil": math.Ceil,
}

func (p *parser) primary() (float64, error) {
	switch c := p.peek(); {
	case c == '(':
		if err := p.enter(); err != nil {
			return 0, err
		}
		defer p.leave()
		p.at++
		v, _, err := p.expr()
		if err != nil {
			return 0, err
		}
		if p.peek() != ')' {
			return 0, errSyntax
		}
		p.at++
		return v, nil
	case c >= '0' && c <= '9' || c == '.':
		start := p.at
		for p.at < len(p.in) && (p.in[p.at] >= '0' && p.in[p.at] <= '9' || p.in[p.at] == '.' || p.in[p.at] == ',') {
			p.at++
		}
		raw := string(p.in[start:p.at])
		if strings.Contains(raw, ",") && !thousands.MatchString(raw) {
			return 0, errSyntax // "1,5" may be a decimal comma: not guessed at
		}
		v, err := strconv.ParseFloat(strings.ReplaceAll(raw, ",", ""), 64)
		if err != nil {
			return 0, errSyntax
		}
		return v, nil
	case unicode.IsLetter(c):
		name := p.word()
		switch name {
		case "pi", "π":
			return math.Pi, nil
		case "e":
			return math.E, nil
		}
		fn, known := functions[name]
		if !known || p.peek() != '(' {
			return 0, errSyntax
		}
		if err := p.enter(); err != nil {
			return 0, err
		}
		defer p.leave()
		p.at++
		arg, _, err := p.expr()
		if err != nil {
			return 0, err
		}
		if p.peek() != ')' {
			return 0, errSyntax
		}
		p.at++
		p.calls = true
		return fn(arg), nil
	}
	return 0, errSyntax
}
