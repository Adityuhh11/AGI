package clipboard

import (
	"os/exec"
	"strings"
	"testing"
)

// These use the real pasteboard through pbcopy, so they run where there is a login session (the CI runner has one).
func pbcopy(t *testing.T, text string) {
	t.Helper()
	cmd := exec.Command("pbcopy")
	cmd.Stdin = strings.NewReader(text)
	if err := cmd.Run(); err != nil {
		t.Skipf("pbcopy is not available here: %v", err)
	}
}

func TestACopyChangesTheSequenceAndItsTextIsRead(t *testing.T) {
	src := NewSource()
	before := src.Sequence()
	pbcopy(t, "hello from the pasteboard")
	if src.Sequence() == before {
		t.Fatal("the sequence did not change after a copy")
	}
	clip, err := src.Read()
	if err != nil || clip.Text != "hello from the pasteboard" {
		t.Fatalf("Read = %+v, %v", clip, err)
	}
	if clip.Private {
		t.Error("a plain copy was read as private")
	}
	if strings.EqualFold(clip.Owner, selfName) {
		t.Errorf("a copy made by another program was said to be this program's own (owner %q)", clip.Owner)
	}
}

func TestACopyThisProgramMadeIsKnownAsItsOwn(t *testing.T) {
	src := NewSource()
	OwnCopy(func() bool { pbcopy(t, "pasted by omni"); return true }) // as the paster writes
	clip, err := src.Read()
	if err != nil || !strings.EqualFold(clip.Owner, selfName) {
		t.Fatalf("Read = %+v, %v: the owner must be this program, so the history leaves it out", clip, err)
	}
	pbcopy(t, "copied by hand afterwards")
	clip, _ = src.Read()
	if strings.EqualFold(clip.Owner, selfName) {
		t.Fatal("a later copy kept being treated as this program's own")
	}
}

func TestATextOverTheLimitIsRefusedNotCut(t *testing.T) {
	pbcopy(t, strings.Repeat("x", MaxText+1))
	if _, err := NewSource().Read(); err != ErrTooLong {
		t.Fatalf("Read error = %v, want ErrTooLong", err)
	}
}
