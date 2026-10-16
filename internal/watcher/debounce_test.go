package watcher

import (
	"sync"
	"testing"
	"time"
)

type calls struct {
	mu   sync.Mutex
	keys []string
}

func (c *calls) add(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.keys = append(c.keys, key)
}

func (c *calls) get() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.keys...)
}

func TestABurstBecomesOneCallAfterItGoesQuiet(t *testing.T) {
	var got calls
	d := newDebouncer(40*time.Millisecond, time.Second, got.add)
	defer d.stop()
	for range 10 {
		d.poke("a")
		time.Sleep(10 * time.Millisecond) // a burst: each event is sooner than the quiet time
	}
	if n := len(got.get()); n != 0 {
		t.Fatalf("%d calls while the burst was still going", n)
	}
	time.Sleep(120 * time.Millisecond)
	if keys := got.get(); len(keys) != 1 || keys[0] != "a" {
		t.Fatalf("calls = %v, want one for a", keys)
	}
}

func TestAnEndlessBurstStillFiresAtTheLongestWait(t *testing.T) {
	var got calls
	d := newDebouncer(50*time.Millisecond, 120*time.Millisecond, got.add)
	defer d.stop()
	stop := time.After(300 * time.Millisecond)
	for done := false; !done; {
		d.poke("busy")
		select {
		case <-stop:
			done = true
		case <-time.After(10 * time.Millisecond): // never quiet for 50 ms
		}
	}
	if len(got.get()) < 1 {
		t.Fatal("a burst that never goes quiet must still be answered once the longest wait is up")
	}
}

func TestKeysAreIndependentAndAFinishedBurstCanStartAgain(t *testing.T) {
	var got calls
	d := newDebouncer(20*time.Millisecond, time.Second, got.add)
	defer d.stop()
	d.poke("a")
	d.poke("b")
	time.Sleep(80 * time.Millisecond)
	d.poke("a")
	time.Sleep(80 * time.Millisecond)
	keys := got.get()
	count := map[string]int{}
	for _, k := range keys {
		count[k]++
	}
	if count["a"] != 2 || count["b"] != 1 || len(keys) != 3 {
		t.Fatalf("calls = %v, want a twice and b once", keys)
	}
}

func TestNothingFiresAfterStop(t *testing.T) {
	var got calls
	d := newDebouncer(20*time.Millisecond, time.Second, got.add)
	d.poke("a")
	d.stop()
	d.poke("b")
	time.Sleep(80 * time.Millisecond)
	if keys := got.get(); len(keys) != 0 {
		t.Fatalf("calls after stop: %v", keys)
	}
}
