package calc

import (
	"regexp"
	"strconv"
	"strings"
)

// A unit is a name, its dimension and how many of the dimension's base unit it is (metres, grams, litres, bytes, seconds,
// metres per second). Temperature has its own rules and is handled apart.
type unit struct {
	dimension string
	factor    float64
}

var units = map[string]unit{}

func init() {
	add := func(dimension string, factor float64, names ...string) {
		for _, n := range names {
			units[n] = unit{dimension, factor}
		}
	}
	add("length", 0.001, "mm", "millimeter", "millimetre")
	add("length", 0.01, "cm", "centimeter", "centimetre")
	add("length", 1, "m", "meter", "metre")
	add("length", 1000, "km", "kilometer", "kilometre")
	add("length", 0.0254, "in", "inch", "inches", "\"")
	add("length", 0.3048, "ft", "foot", "feet", "'")
	add("length", 0.9144, "yd", "yard")
	add("length", 1609.344, "mi", "mile", "miles")
	add("length", 1852, "nmi", "nauticalmile")

	add("mass", 0.001, "mg", "milligram")
	add("mass", 1, "g", "gram")
	add("mass", 1000, "kg", "kilogram", "kilo")
	add("mass", 1000000, "t", "tonne")
	add("mass", 28.349523125, "oz", "ounce")
	add("mass", 453.59237, "lb", "lbs", "pound")
	add("mass", 6350.29318, "st", "stone")

	add("volume", 0.001, "ml", "milliliter", "millilitre")
	add("volume", 1, "l", "liter", "litre")
	add("volume", 0.00492892159375, "tsp", "teaspoon")
	add("volume", 0.01478676478125, "tbsp", "tablespoon")
	add("volume", 0.0295735295625, "floz")
	add("volume", 0.2365882365, "cup")
	add("volume", 0.473176473, "pt", "pint")
	add("volume", 0.946352946, "qt", "quart")
	add("volume", 3.785411784, "gal", "gallon")

	add("data", 1, "b", "byte")
	add("data", 1e3, "kb", "kilobyte")
	add("data", 1e6, "mb", "megabyte")
	add("data", 1e9, "gb", "gigabyte")
	add("data", 1e12, "tb", "terabyte")
	add("data", 1024, "kib")
	add("data", 1024*1024, "mib")
	add("data", 1024*1024*1024, "gib")
	add("data", 1024*1024*1024*1024, "tib")

	add("time", 0.001, "ms", "millisecond")
	add("time", 1, "s", "sec", "second")
	add("time", 60, "min", "minute")
	add("time", 3600, "h", "hr", "hour")
	add("time", 86400, "d", "day")
	add("time", 604800, "wk", "week")

	add("speed", 1, "m/s", "mps")
	add("speed", 1000.0/3600, "km/h", "kph", "kmh")
	add("speed", 0.44704, "mph")
	add("speed", 1852.0/3600, "kn", "knot")

	add("area", 1, "m2", "sqm")
	add("area", 1e6, "km2", "sqkm")
	add("area", 10000, "ha", "hectare")
	add("area", 4046.8564224, "acre")
	add("area", 0.09290304, "ft2", "sqft")
}

// canonical is a unit as written, lowered, without a dot, spaces or a plural s, and with the usual spellings folded.
func canonical(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	s = strings.NewReplacer(".", "", " ", "", "²", "2", "°", "").Replace(s)
	switch s {
	case "celsius", "centigrade":
		return "c"
	case "fahrenheit":
		return "f"
	case "kelvin":
		return "k"
	}
	if _, ok := units[s]; ok {
		return s
	}
	if strings.HasSuffix(s, "s") {
		if _, ok := units[s[:len(s)-1]]; ok {
			return s[:len(s)-1]
		}
	}
	return s
}

var temperatures = map[string]bool{"c": true, "f": true, "k": true}

func toCelsius(v float64, from string) float64 {
	switch from {
	case "f":
		return (v - 32) * 5 / 9
	case "k":
		return v - 273.15
	}
	return v
}

func fromCelsius(c float64, to string) float64 {
	switch to {
	case "f":
		return c*9/5 + 32
	case "k":
		return c + 273.15
	}
	return c
}

var leftSide = regexp.MustCompile(`^(-?\d[\d,]*(?:\.\d+)?|-?\.\d+)\s*(.+)$`)

// convert answers "5 km in miles", "72 f to c", "3 gb as mb". The separator is the last word among in, to, as and into
// that leaves a number and a unit on the left and a unit on the right ("5 in in cm" splits at the second).
func convert(text string) (Result, bool) {
	words := strings.Fields(text)
	for i := len(words) - 2; i >= 1; i-- {
		switch strings.ToLower(words[i]) {
		case "in", "to", "as", "into":
		default:
			continue
		}
		if r, ok := convertParts(strings.Join(words[:i], " "), strings.Join(words[i+1:], " ")); ok {
			return r, true
		}
	}
	return Result{}, false
}

func convertParts(left, right string) (Result, bool) {
	m := leftSide.FindStringSubmatch(left)
	if m == nil {
		return Result{}, false
	}
	value, err := strconv.ParseFloat(strings.ReplaceAll(m[1], ",", ""), 64)
	if err != nil {
		return Result{}, false
	}
	from, to := canonical(m[2]), canonical(right)
	var out float64
	switch {
	case temperatures[from] && temperatures[to]:
		out = fromCelsius(toCelsius(value, from), to)
	default:
		a, okA := units[from]
		b, okB := units[to]
		if !okA || !okB || a.dimension != b.dimension {
			return Result{}, false
		}
		out = value * a.factor / b.factor
	}
	return Result{
		Title: plain(value) + " " + strings.TrimSpace(m[2]) + " = " + pretty(out) + " " + strings.TrimSpace(right),
		Copy:  plain(out),
	}, true
}
