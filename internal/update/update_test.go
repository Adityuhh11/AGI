package update

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
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

const goodSum = "6f87d2b84cc83e6ecc88dc825250b6501f0a77774de585e6b6a198aca3a6ed5d"

func TestTheAssetIsTheWindowsZipWithItsDigest(t *testing.T) {
	files := []githubFile{
		{Name: "notes.txt", URL: "https://x/notes.txt", Digest: "sha256:" + goodSum},
		{Name: "agi-0.3.0-windows-x64.zip", URL: "https://github.com/Adityuhh11/AGI/releases/download/v0.3.0/agi-0.3.0-windows-x64.zip", Size: 5653358, Digest: "sha256:" + strings.ToUpper(goodSum)},
	}
	got := assetOf("0.3.0", files)
	if got.Name != "agi-0.3.0-windows-x64.zip" || got.Size != 5653358 || got.SHA256 != goodSum || !strings.HasSuffix(got.URL, "/agi-0.3.0-windows-x64.zip") {
		t.Fatalf("asset %+v (the digest is kept in lower case)", got)
	}
}

func TestAReleaseWithoutAZipThatCanBeCheckedHasNoAsset(t *testing.T) {
	zip := func(digest string) []githubFile {
		return []githubFile{{Name: "agi-0.3.0-windows-x64.zip", URL: "https://x/z", Digest: digest}}
	}
	for name, files := range map[string][]githubFile{
		"no files":              nil,
		"another version's zip": {{Name: "agi-0.2.0-windows-x64.zip", Digest: "sha256:" + goodSum}},
		"no digest":             zip(""),
		"another algorithm":     zip("sha512:" + goodSum),
		"too short":             zip("sha256:abcd"),
		"not hex":               zip("sha256:" + strings.Repeat("z", 64)),
	} {
		if got := assetOf("0.3.0", files); got != (Asset{}) {
			t.Errorf("%s: asset %+v, want none (nothing is installed that cannot be checked)", name, got)
		}
	}
}

func TestCheckKeepsTheAssetOfTheNewerRelease(t *testing.T) {
	c := checker(t, "0.2.0", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"tag_name":"v0.3.0","html_url":"https://github.com/Adityuhh11/AGI/releases/tag/v0.3.0","assets":[{"name":"agi-0.3.0-windows-x64.zip","size":5653358,"digest":"sha256:` + goodSum + `","browser_download_url":"https://github.com/Adityuhh11/AGI/releases/download/v0.3.0/agi-0.3.0-windows-x64.zip"}]}`))
	})
	if _, newer, err := c.Check(context.Background()); err != nil || !newer {
		t.Fatalf("newer=%v err=%v", newer, err)
	}
	found, ok := c.Found()
	if !ok || found.Asset.SHA256 != goodSum || found.Asset.Size != 5653358 {
		t.Fatalf("found %+v", found)
	}
}
