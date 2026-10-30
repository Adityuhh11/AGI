package enginesetup

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

type entry struct {
	name, body, link string
	typeflag         byte
	mode             int64
}

func makeTarGz(t *testing.T, entries ...entry) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "a.tar.gz")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zipped := gzip.NewWriter(file)
	writer := tar.NewWriter(zipped)
	for _, e := range entries {
		header := &tar.Header{Name: e.name, Typeflag: e.typeflag, Mode: e.mode, Linkname: e.link, Size: int64(len(e.body))}
		if err := writer.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write([]byte(e.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := zipped.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestUntarDropsTheWrappingFolderAndKeepsFilesAndTheirContents(t *testing.T) {
	archive := makeTarGz(t,
		entry{name: "llama-b1/", typeflag: tar.TypeDir, mode: 0o755},
		entry{name: "llama-b1/llama-server", body: "binary", typeflag: tar.TypeReg, mode: 0o755},
		entry{name: "llama-b1/lib/x.dylib", body: "lib", typeflag: tar.TypeReg, mode: 0o644},
	)
	dir := t.TempDir()
	if err := Untar(archive, dir, true); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"llama-server": "binary", filepath.Join("lib", "x.dylib"): "lib"} {
		got, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || string(got) != want {
			t.Errorf("%s = %q, %v; want %q", name, got, err, want)
		}
	}
	if runtime.GOOS != "windows" { // Windows has no executable bit
		if info, _ := os.Stat(filepath.Join(dir, "llama-server")); info.Mode().Perm()&0o100 == 0 {
			t.Error("the executable bit was lost, and a Mac cannot start the server without it")
		}
	}
}

func TestUntarWithoutStripKeepsTheFolder(t *testing.T) {
	archive := makeTarGz(t, entry{name: "top/f.txt", body: "x", typeflag: tar.TypeReg, mode: 0o644})
	dir := t.TempDir()
	if err := Untar(archive, dir, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "top", "f.txt")); err != nil {
		t.Fatal(err)
	}
}

func TestUntarKeepsALibrarysVersionLinks(t *testing.T) {
	archive := makeTarGz(t,
		entry{name: "d/libx.0.dylib", body: "lib", typeflag: tar.TypeReg, mode: 0o755},
		entry{name: "d/libx.dylib", link: "libx.0.dylib", typeflag: tar.TypeSymlink, mode: 0o755},
	)
	dir := t.TempDir()
	if err := Untar(archive, dir, true); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "libx.dylib"))
	if err != nil {
		t.Skipf("this system will not make symbolic links without special rights: %v", err) // Windows, unless run as developer
	}
	if string(got) != "lib" {
		t.Fatalf("the link reads %q", got)
	}
}

func TestUntarRefusesAnythingThatLeavesTheFolder(t *testing.T) {
	for name, e := range map[string]entry{
		"a path":   {name: "d/../../escape.txt", body: "x", typeflag: tar.TypeReg, mode: 0o644},
		"a link":   {name: "d/l", link: "../../outside", typeflag: tar.TypeSymlink, mode: 0o755},
		"an slash": {name: "/etc/passwd-ish", body: "x", typeflag: tar.TypeReg, mode: 0o644},
	} {
		dir := filepath.Join(t.TempDir(), "into")
		if err := Untar(makeTarGz(t, e), dir, true); err == nil {
			t.Errorf("%s: an entry outside the folder was extracted", name)
		}
	}
}

func TestUnpackTakesATarballFromTheNetworkOnceAndStripsItsFolder(t *testing.T) {
	archive := makeTarGz(t, entry{name: "uv-x/uv", body: "the tool", typeflag: tar.TypeReg, mode: 0o755})
	data, err := os.ReadFile(archive)
	if err != nil {
		t.Fatal(err)
	}
	srv := newFileServer(t, map[string][]byte{"/uv.tar.gz": data})
	art := artifactOf(srv.URL+"/uv.tar.gz", data)
	art.Strip = true
	dir := filepath.Join(t.TempDir(), "tools")
	if err := (Downloader{}).Unpack(t.Context(), art, dir, func(int64, int64) {}); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(dir, "uv")); err != nil || string(got) != "the tool" {
		t.Fatalf("uv = %q, %v", got, err)
	}
	if err := (Downloader{}).Unpack(t.Context(), art, dir, func(int64, int64) {}); err != nil {
		t.Fatal(err)
	}
	if n := len(srv.requests()); n != 1 {
		t.Errorf("the second unpack fetched again (%d requests)", n)
	}
}

func TestUntarRefusesALinkToAnAbsolutePlaceAndAKindOfEntryItDoesNotUnpack(t *testing.T) {
	for name, e := range map[string]entry{
		"an absolute link": {name: "d/l", link: "/etc", typeflag: tar.TypeSymlink, mode: 0o755},
		"a hard link":      {name: "d/h", link: "d/other", typeflag: tar.TypeLink, mode: 0o644},
	} {
		if err := Untar(makeTarGz(t, e), filepath.Join(t.TempDir(), "into"), true); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
}

// A link that points back at the folder makes the same place reachable under many names, so a later link written
// "inside" it can be judged by its name while it really sits higher up; nothing may be written through one.
func TestUntarDoesNotWriteThroughALinkToEscape(t *testing.T) {
	outer := t.TempDir()
	dir := filepath.Join(outer, "a", "b", "into")
	archive := makeTarGz(t,
		entry{name: "p", link: ".", typeflag: tar.TypeSymlink, mode: 0o755},
		entry{name: "p/p/p/up", link: "../../..", typeflag: tar.TypeSymlink, mode: 0o755},
		entry{name: "p/p/p/up/escaped.txt", body: "x", typeflag: tar.TypeReg, mode: 0o644},
	)
	_ = Untar(archive, dir, false)
	if _, err := os.Stat(filepath.Join(outer, "escaped.txt")); err == nil {
		t.Fatal("a file was written outside the folder, through a chain of links")
	}
}
