package web

import (
	"reflect"
	"testing"
)

func TestAFirefoxBasedBrowserSearchesByAddressOnAMac(t *testing.T) {
	f := &fake{installed: map[string]string{"zen": "/Applications/Zen.app"}}
	if err := newWeb(f, "zen", "").Search("rust lifetimes"); err != nil {
		t.Fatal(err)
	}
	if f.exe != "/Applications/Zen.app" || !reflect.DeepEqual(f.args, []string{"https://duckduckgo.com/?q=rust+lifetimes"}) {
		t.Fatalf("started %q %q: open --args would be ignored by a browser that is already running, so the address template is used", f.exe, f.args)
	}
}

func TestABrowserCanBeGivenAsABundlePathOrANameInAnyForm(t *testing.T) {
	for _, name := range []string{"Zen", "zen.app", "/Applications/Zen.app"} {
		if got := browserName(name); got != "zen" {
			t.Errorf("browserName(%q) = %q, want zen", name, got)
		}
	}
	f := &fake{}
	if err := newWeb(f, "/Applications/Zen.app", "").Search("x"); err != nil || f.exe != "/Applications/Zen.app" {
		t.Fatalf("a path is used as it is, got %q, %v", f.exe, err)
	}
}

func TestEveryBrowserOfferedHasAnApp(t *testing.T) {
	for _, name := range knownBrowsers {
		if browserApps[name] == "" {
			t.Errorf("%s is offered but has no app name", name)
		}
	}
}
