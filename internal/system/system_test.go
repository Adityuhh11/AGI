package system

import "testing"

func TestEveryOfferedActionIsKnownAndAnUnknownOneIsRefused(t *testing.T) {
	if len(Commands) == 0 {
		t.Fatal("this system offers no actions")
	}
	for _, c := range Commands {
		if _, ok := actions[c.ID]; !ok {
			t.Errorf("%s has no action", c.ID)
		}
	}
	if err := Run("sys:shutdown"); err == nil {
		t.Error("shutting down must not be offered")
	}
}
