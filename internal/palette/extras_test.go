package palette

import (
	"errors"
	"strings"
	"testing"
)

func TestASumIsAnsweredFirstAndEnterCopiesTheAnswer(t *testing.T) {
	s, dismissed := newService(t, "Calculator")
	var copied []string
	With(s, Extras{Copy: func(text string) error { copied = append(copied, text); return nil }})

	answer := s.Query("12*7+3", ModeAll)
	if len(answer.Items) == 0 || answer.Items[0].Kind != "calc" || answer.Items[0].Title != "87" {
		t.Fatalf("items = %+v", answer.Items)
	}
	if err := s.Do(answer.Items[0].ID, "open", "12*7+3"); err != nil {
		t.Fatal(err)
	}
	if len(copied) != 1 || copied[0] != "87" || *dismissed != 1 {
		t.Errorf("copied %v, dismissed %d", copied, *dismissed)
	}
}

func TestAConversionRowCopiesTheNumberAlone(t *testing.T) {
	s, _ := newService(t)
	var copied string
	With(s, Extras{Copy: func(text string) error { copied = text; return nil }})
	item := s.Query("5 km in miles", ModeAll).Items[0]
	if !strings.Contains(item.Title, "5 km = 3.10") {
		t.Fatalf("title = %q", item.Title)
	}
	if err := s.Do(item.ID, "open", ""); err != nil || copied != "3.106855961" {
		t.Errorf("copied %q, err %v", copied, err)
	}
}

func TestOrdinaryTextAndOtherModesGetNoCalculatorRow(t *testing.T) {
	s, _ := newService(t, "Visual Studio Code")
	for _, text := range []string{"vsc", "2024", "chrome"} {
		if got := kinds(s.Query(text, ModeAll)); strings.Contains(got, "calc") {
			t.Errorf("%q got a calculator row: %s", text, got)
		}
	}
	if got := kinds(s.Query("2+2", ModeApps)); strings.Contains(got, "calc") {
		t.Errorf("the Apps mode answered a sum: %s", got)
	}
}

func TestACalculatorRowFailsClearlyWithoutAClipboard(t *testing.T) {
	s, _ := newService(t)
	if err := s.Do("calc:4", "open", ""); err == nil {
		t.Error("no clipboard, no error")
	}
	With(s, Extras{Copy: func(string) error { return errors.New("locked") }})
	if err := s.Do("calc:4", "open", ""); err == nil || !strings.Contains(err.Error(), "locked") {
		t.Errorf("err = %v", err)
	}
	if err := s.Do("calc:4", "reveal", ""); err == nil {
		t.Error("a calculator row can only be opened")
	}
}

func TestSystemRowsAreFoundByNameAndRunAfterTheBarIsClosed(t *testing.T) {
	s, dismissed := newService(t)
	var ran []string
	var dismissedBefore int
	With(s, Extras{RunSystem: func(id string) error { dismissedBefore = *dismissed; ran = append(ran, id); return nil }})

	for text, id := range typedForSystemRows {
		found := false
		for _, item := range s.Query(text, ModeAll).Items {
			found = found || item.ID == id
		}
		if !found {
			t.Errorf("%q did not offer %s", text, id)
		}
	}
	if err := s.Do("sys:lock", "open", ""); err != nil || len(ran) != 1 || ran[0] != "sys:lock" || dismissedBefore != 1 {
		t.Errorf("ran %v, dismissed before running %d, err %v", ran, dismissedBefore, err)
	}
	if got := kinds(s.Query("lo", ModeAll)); strings.Contains(got, "system") {
		t.Errorf("two letters offered a system row: %s", got)
	}
	if got := kinds(s.Query("shutdown", ModeAll)); strings.Contains(got, "system") {
		t.Errorf("shutting down must not be offered: %s", got)
	}
}

func TestASystemRowCanBePinnedAndFoundAgain(t *testing.T) {
	s, _ := newService(t)
	item, ok := s.itemFor("sys:lock")
	if !ok || item.Kind != "system" {
		t.Fatalf("itemFor = %+v, %v", item, ok)
	}
}
