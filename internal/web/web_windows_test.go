package web

import (
	"reflect"
	"testing"
)

func TestAFirefoxBasedBrowserSearchesWithItsOwnEngine(t *testing.T) {
	f := &fake{installed: map[string]string{"zen": `C:\Program Files\Zen Browser\zen.exe`}}
	if err := newWeb(f, "zen", "").Search("rust lifetimes & borrowing"); err != nil {
		t.Fatal(err)
	}
	if f.exe != `C:\Program Files\Zen Browser\zen.exe` || !reflect.DeepEqual(f.args, []string{"-search", "rust lifetimes & borrowing"}) {
		t.Fatalf("started %q %q", f.exe, f.args)
	}
}

func TestABrowserCanBeGivenAsAPathOrNameInAnyForm(t *testing.T) {
	for _, name := range []string{"Zen", "zen.exe", `C:\Program Files\Zen Browser\zen.exe`} {
		if got := browserName(name); got != "zen" {
			t.Errorf("browserName(%q) = %q, want zen", name, got)
		}
	}
	f := &fake{}
	if err := newWeb(f, `C:\Apps\zen.exe`, "").Search("x"); err != nil || f.exe != `C:\Apps\zen.exe` {
		t.Fatalf("a path is used as it is, got %q, %v", f.exe, err)
	}
}
