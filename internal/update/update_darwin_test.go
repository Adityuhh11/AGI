package update

import (
	"context"
	"net/http"
	"testing"
)

func TestAMacIsNeverOfferedAnInstallButStillLearnsOfANewRelease(t *testing.T) {
	files := []githubFile{{Name: "omni-0.3.0-windows-x64.zip", URL: "https://x/z", Size: 1, Digest: "sha256:" + goodSum}}
	if got := assetOf("0.3.0", files); got != (Asset{}) {
		t.Fatalf("assetOf = %+v: a Mac has no one-click update yet, and must not be handed a Windows program", got)
	}
	c := checker(t, "0.2.0", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"tag_name":"v0.3.0","html_url":"https://github.com/Adityuhh11/Omni/releases/tag/v0.3.0","assets":[]}`))
	})
	if _, newer, err := c.Check(context.Background()); err != nil || !newer {
		t.Fatalf("newer=%v err=%v", newer, err)
	}
	if found, ok := c.Found(); !ok || found.Asset.URL != "" || found.Version != "0.3.0" {
		t.Fatalf("found %+v: the release page is what a Mac user is sent to", found)
	}
}
