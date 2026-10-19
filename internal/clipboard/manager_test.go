package clipboard

import (
	"errors"
	"testing"
	"time"
)

func newManager(t *testing.T) (*Manager, *scene) {
	t.Helper()
	store, _ := open(t)
	s := &scene{target: 42}
	m := NewManager(store, NewWatcher(&fakeSource{}, store, func() bool { return true }, func() []string { return nil }, "agi"), s.paster(), func() bool { return true })
	m.now = func() time.Time { return t0.Add(time.Hour) }
	return m, s
}

func TestPastingAnEntryTypesItAndMovesItToTheTop(t *testing.T) {
	m, s := newManager(t)
	add(t, m.store, "old", 0)
	add(t, m.store, "new", 1)
	old := m.Search("old", 1)[0]
	s.comeBackAfter = 0
	if err := m.Paste(old.ID); err != nil {
		t.Fatal(err)
	}
	if got := order(s); got != "set:old,release,ctrl+v" {
		t.Fatalf("events %q", got)
	}
	if got := texts(m.Search("", 10)); got != "old|new" {
		t.Fatalf("history %q: a pasted entry goes to the top", got)
	}
}

func TestCopyingAnEntryDoesNotType(t *testing.T) {
	m, s := newManager(t)
	add(t, m.store, "x", 0)
	if err := m.Copy(m.Search("", 1)[0].ID); err != nil {
		t.Fatal(err)
	}
	if got := order(s); got != "set:x,release" {
		t.Fatalf("events %q", got)
	}
}

func TestAnEntryThatIsGoneIsAnErrorAndNothingIsTyped(t *testing.T) {
	m, s := newManager(t)
	if err := m.Paste(99); err == nil {
		t.Fatal("no error")
	}
	if err := m.Copy(99); err == nil {
		t.Fatal("no error")
	}
	if len(s.events) != 0 {
		t.Fatalf("events %q", order(s))
	}
}

func TestAPasteThatCannotBeTypedStillReportsWhy(t *testing.T) {
	m, s := newManager(t)
	s.elevated = true
	add(t, m.store, "x", 0)
	if err := m.Paste(m.Search("", 1)[0].ID); !errors.Is(err, ErrElevated) {
		t.Fatalf("err = %v", err)
	}
}

func TestDeleteAndClearForgetEntries(t *testing.T) {
	m, _ := newManager(t)
	add(t, m.store, "a", 0)
	add(t, m.store, "b", 1)
	if err := m.Delete(m.Search("a", 1)[0].ID); err != nil || m.Len() != 1 {
		t.Fatalf("after Delete: %d entries, %v", m.Len(), err)
	}
	if err := m.Clear(); err != nil || m.Len() != 0 {
		t.Fatalf("after Clear: %d entries, %v", m.Len(), err)
	}
}
