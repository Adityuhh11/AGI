package update

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestTheAssetIsTheWindowsZipWithItsDigest(t *testing.T) {
	files := []githubFile{
		{Name: "notes.txt", URL: "https://x/notes.txt", Digest: "sha256:" + goodSum},
		{Name: "omni-0.3.0-windows-x64.zip", URL: "https://github.com/Adityuhh11/Omni/releases/download/v0.3.0/omni-0.3.0-windows-x64.zip", Size: 5653358, Digest: "sha256:" + strings.ToUpper(goodSum)},
	}
	got := assetOf("0.3.0", files)
	if got.Name != "omni-0.3.0-windows-x64.zip" || got.Size != 5653358 || got.SHA256 != goodSum || !strings.HasSuffix(got.URL, "/omni-0.3.0-windows-x64.zip") {
		t.Fatalf("asset %+v (the digest is kept in lower case)", got)
	}
}

func TestAReleaseWithoutAZipThatCanBeCheckedHasNoAsset(t *testing.T) {
	zip := func(digest string) []githubFile {
		return []githubFile{{Name: "omni-0.3.0-windows-x64.zip", URL: "https://x/z", Digest: digest}}
	}
	for name, files := range map[string][]githubFile{
		"no files":              nil,
		"another version's zip": {{Name: "omni-0.2.0-windows-x64.zip", Digest: "sha256:" + goodSum}},
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
		w.Write([]byte(`{"tag_name":"v0.3.0","html_url":"https://github.com/Adityuhh11/Omni/releases/tag/v0.3.0","assets":[{"name":"omni-0.3.0-windows-x64.zip","size":5653358,"digest":"sha256:` + goodSum + `","browser_download_url":"https://github.com/Adityuhh11/Omni/releases/download/v0.3.0/omni-0.3.0-windows-x64.zip"}]}`))
	})
	if _, newer, err := c.Check(context.Background()); err != nil || !newer {
		t.Fatalf("newer=%v err=%v", newer, err)
	}
	found, ok := c.Found()
	if !ok || found.Asset.SHA256 != goodSum || found.Asset.Size != 5653358 {
		t.Fatalf("found %+v", found)
	}
}
