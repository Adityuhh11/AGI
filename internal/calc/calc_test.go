package calc

import (
	"strings"
	"testing"
)

func TestAnswerArithmetic(t *testing.T) {
	tests := []struct {
		text, copy string
	}{
		{"12*7+3", "87"},
		{"2^10", "1024"},
		{"2^3^2", "512"},
		{"(1+2)*3", "9"},
		{"-4 + 10", "6"},
		{"10/4", "2.5"},
		{"1/3", "0.3333333333"},
		{"3 x 4", "12"},
		{"3 × 4", "12"},
		{"= 5*5", "25"},
		{"15% of 80", "12"},
		{"100 + 10%", "110"},
		{"100 - 25%", "75"},
		{"50%", "0.5"},
		{"sqrt(144)", "12"},
		{"sqrt(2) * sqrt(2)", "2"},
		{"abs(-3.5)", "3.5"},
		{"round(2.5)", "3"},
		{"sin(90)", "1"},
		{"2*pi", "6.283185307"},
		{"1,000 * 3", "3000"},
		{"0.1 + 0.2", "0.3"},
		{"1e3", ""},
		{"2 * 3 * 4 * 5", "120"},
		{"0 * 5", "0"},
		{"10 - 12", "-2"},
		{"12/4", "3"},
		{"10-12+1", "-1"},
		{"-2^2", "-4"},
		{"(-2)^2", "4"},
		{"2^-2", "0.25"},
		{"2^3^2", "512"},
		{"-3 * -3", "9"},
		{"10% + 5%", "0.15"},
		{"200 + 10% + 5%", "231"},
		{"1,000 + 1", "1001"},
		{"12,345,678.5 * 2", "24691357"},
		{"0 * -5", "0"},
	}
	for _, tt := range tests {
		got, ok := Answer(tt.text)
		if tt.copy == "" {
			if ok {
				t.Errorf("Answer(%q) = %+v, want no answer", tt.text, got)
			}
			continue
		}
		if !ok || got.Copy != tt.copy {
			t.Errorf("Answer(%q) = %+v, %v; want copy %q", tt.text, got, ok, tt.copy)
		}
	}
}

func TestAnswerLeavesOrdinarySearchesAlone(t *testing.T) {
	for _, text := range []string{"", "   ", "42", "2024", "3.14", "chrome", "visual studio", "note to self", "7zip", "5 apples", "a+b", "x", "e", "-", "()", "2 +", "1/0", "sqrt(", "sqrt 4", "10 km to", "1,5+1,5", "1,00 * 2", "2024-01-05", "10-12", "1/2/2026", "2026-10-26-2"} {
		if got, ok := Answer(text); ok {
			t.Errorf("Answer(%q) = %+v, want no answer", text, got)
		}
	}
}

func TestAnswerShowsThousandsApartButCopiesPlainNumbers(t *testing.T) {
	got, ok := Answer("1234 * 1000")
	if !ok || got.Title != "1,234,000" || got.Copy != "1234000" {
		t.Fatalf("got %+v, %v", got, ok)
	}
	got, _ = Answer("-1234.5 * 10")
	if got.Title != "-12,345" || got.Copy != "-12345" {
		t.Fatalf("got %+v", got)
	}
	got, _ = Answer("1000 + 1")
	if got.Title != "1001" {
		t.Fatalf("four digits are left alone: %+v", got)
	}
}

func TestConversions(t *testing.T) {
	tests := []struct {
		text, copy string
	}{
		{"5 km in miles", "3.106855961"},
		{"5km to mi", "3.106855961"},
		{"1 mile in km", "1.609344"},
		{"10 miles to kilometers", "16.09344"},
		{"72 f in c", "22.22222222"},
		{"0 c to f", "32"},
		{"100 celsius in kelvin", "373.15"},
		{"3 gb in mb", "3000"},
		{"3 gib to mib", "3072"},
		{"2 h to min", "120"},
		{"90 min as hours", "1.5"},
		{"1 lb to oz", "16"},
		{"100 kg in lbs", "220.4622622"},
		{"5 in in cm", "12.7"},
		{"12 inches to feet", "1"},
		{"1 gal in liters", "3.785411784"},
		{"60 mph to km/h", "96.56064"},
		{"1 acre to m2", "4046.856422"},
		{"1,000 m to km", "1"},
	}
	for _, tt := range tests {
		got, ok := Answer(tt.text)
		if !ok || got.Copy != tt.copy {
			t.Errorf("Answer(%q) = %+v, %v; want copy %q", tt.text, got, ok, tt.copy)
		}
	}
}

func TestConversionsRefuseWhatDoesNotConvert(t *testing.T) {
	for _, text := range []string{"5 km in kg", "5 apples in pears", "km in miles", "5 in", "to km", "5 c in m"} {
		if got, ok := Answer(text); ok {
			t.Errorf("Answer(%q) = %+v, want no answer", text, got)
		}
	}
}

func TestConversionTitleReadsAsTheSentence(t *testing.T) {
	got, _ := Answer("5 km in miles")
	if got.Title != "5 km = 3.106855961 miles" {
		t.Fatalf("title = %q", got.Title)
	}
}

func TestAWallOfBracketsOrSignsIsRefusedNotParsedToTheBottom(t *testing.T) {
	for _, text := range []string{strings.Repeat("(", 5000) + "1" + strings.Repeat(")", 5000), strings.Repeat("-", 5000) + "1", strings.Repeat("1+", 5000) + "1"} {
		if got, ok := Answer(text); ok {
			t.Errorf("a %d character expression was answered: %+v", len(text), got)
		}
	}
	if got, ok := Answer(strings.Repeat("(", 20) + "1+1" + strings.Repeat(")", 20)); !ok || got.Copy != "2" {
		t.Errorf("reasonable nesting was refused: %+v, %v", got, ok)
	}
}
