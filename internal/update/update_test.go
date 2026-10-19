package update

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewer(t *testing.T) {
	for _, tc := range []struct {
		latest, current string
		want            bool
	}{
		{"0.3.0", "0.2.0", true},
		{"0.2.0", "0.2.0", false},
		{"0.2.0", "0.3.0", false},
		{"0.10.0", "0.9.0", true}, // numbers, not text
		{"1.0.0", "0.9.9", true},
		{"0.2.0", "0.2.0-rc1", true},  // a release beats its candidate
		{"0.2.0-rc1", "0.2.0", false}, // and not the other way round
		{"0.2.0", "2026.10.18-977d522", true},
		{"0.2.0", "dev", false},
		{"garbage", "0.2.0", false},
	} {
		if got := Newer(tc.latest, tc.current); got != tc.want {
			t.Errorf("Newer(%q, %q) = %v, want %v", tc.latest, tc.current, got, tc.want)
		}
	}
}

func checker(t *testing.T, current string, handler http.HandlerFunc) *Checker {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c := NewChecker(current, nil)
	c.url = srv.URL
	return c
}

func TestCheckFindsANewerRelease(t *testing.T) {
	c := checker(t, "0.2.0", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("no User-Agent: GitHub would refuse the request")
		}
		w.Write([]byte(`{"tag_name":"v0.3.0","html_url":"https://github.com/Adityuhh11/AGI/releases/tag/v0.3.0"}`))
	})
	rel, newer, err := c.Check(context.Background())
	if err != nil || !newer || rel.Version != "0.3.0" {
		t.Fatalf("got %+v newer=%v err=%v", rel, newer, err)
	}
	if found, ok := c.Found(); !ok || found.Version != "0.3.0" {
		t.Fatalf("Found() = %+v, %v", found, ok)
	}
}

func TestCheckForgetsAnUpdateThatIsNoLongerNewer(t *testing.T) {
	tag := "v0.3.0"
	c := checker(t, "0.2.0", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"tag_name":"` + tag + `","html_url":"x"}`))
	})
	c.Check(context.Background())
	tag = "v0.2.0"
	if _, newer, _ := c.Check(context.Background()); newer {
		t.Fatal("0.2.0 reported newer than 0.2.0")
	}
	if _, ok := c.Found(); ok {
		t.Fatal("Found() kept an update the last check did not see")
	}
}

func TestCheckReportsWhyItFailed(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
	}{{"no release yet", http.StatusNotFound}, {"rate limited", http.StatusForbidden}} {
		c := checker(t, "0.2.0", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(tc.status) })
		if _, _, err := c.Check(context.Background()); err == nil {
			t.Errorf("%s: no error", tc.name)
		}
	}
}
