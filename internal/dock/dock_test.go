package dock

import "testing"

func TestTheProgramIsRegularWhileAnyWindowIsOpenAndAccessoryOtherwise(t *testing.T) {
	var calls []bool
	w := &Windows{set: func(regular bool) { calls = append(calls, regular) }}
	w.Opened()
	w.Opened() // a second window changes nothing
	w.Closed()
	if len(calls) != 1 || !calls[0] {
		t.Fatalf("calls = %v, want one switch to regular", calls)
	}
	w.Closed()
	w.Closed() // closed twice: ignored, never negative
	w.Opened()
	want := []bool{true, false, true}
	if len(calls) != len(want) || calls[1] != false || calls[2] != true {
		t.Fatalf("calls = %v, want %v", calls, want)
	}
}
