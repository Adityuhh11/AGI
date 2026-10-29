package files

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"omni/internal/testpath"
)

type launch struct {
	how  string
	path string
	line int
}

type played struct {
	player  string
	path    string
	seconds float64
}

type fakeShell struct {
	launched []launch
	// player is the program the machine has for playing a moment, if any; plays is what it was asked to play.
	player *Player
	plays  []played
}

func (f *fakeShell) SeekPlayer() (Player, bool) {
	if f.player == nil {
		return Player{}, false
	}
	return *f.player, true
}

func (f *fakeShell) Player(p Player, path string, seconds float64) error {
	f.plays = append(f.plays, played{p.Name, path, seconds})
	return nil
}

func (f *fakeShell) Default(path string) error {
	f.launched = append(f.launched, launch{"default", path, 0})
	return nil
}
func (f *fakeShell) Editor(path string, line int) error {
	f.launched = append(f.launched, launch{"editor", path, line})
	return nil
}
func (f *fakeShell) Reveal(path string) error {
	f.launched = append(f.launched, launch{"reveal", path, 0})
	return nil
}

func setup(t *testing.T) (*Service, *fakeShell, string, *int) {
	t.Helper()
	root := t.TempDir()
	shell := &fakeShell{}
	dismissed := 0
	return newService(func() []string { return []string{root} }, func() { dismissed++ }, shell), shell, root, &dismissed
}

func touch(t *testing.T, path string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestScriptsAndCodeOpenInAnEditorNeverTheirDefaultProgram(t *testing.T) {
	// Windows' default action for these is to run them.
	dangerous := []string{
		"a.js", "a.jse", "a.vbs", "a.vbe", "a.wsf", "a.wsh", "a.bat", "a.cmd", "a.ps1", "a.psm1", "a.hta",
		"a.exe", "a.msi", "a.scr", "a.com", "a.lnk", "a.reg", "a.cpl", "a.jar", "a.py", "a.sh",
		"a.docm", "a.xlsm", "a.doc", "a.svg", "a.html", "a.txt", "a.md", "noextension", "a.JS", "a.BAT",
	}
	for _, name := range dangerous {
		if opensInDefaultApp(name) {
			t.Errorf("%s must not open with its default program", name)
		}
	}
	safe := []string{"a.pdf", "a.PDF", "a.png", "a.JPG", "a.jpeg", "a.webp", "a.gif", "a.heic", "a.docx", "a.xlsx", "a.pptx"}
	for _, name := range safe {
		if !opensInDefaultApp(name) {
			t.Errorf("%s should open with its default program", name)
		}
	}
}

func TestOpenRoutesFilesAndClosesTheSearchBar(t *testing.T) {
	svc, shell, root, dismissed := setup(t)
	script := touch(t, filepath.Join(root, "src", "deploy.js"))
	doc := touch(t, filepath.Join(root, "docs", "Week 10 Prep.pdf"))

	if err := svc.Open(script, 42); err != nil {
		t.Fatal(err)
	}
	if err := svc.Open(doc, 7); err != nil {
		t.Fatal(err)
	}
	want := []launch{{"editor", script, 42}, {"default", doc, 0}}
	if len(shell.launched) != 2 || shell.launched[0] != want[0] || shell.launched[1] != want[1] {
		t.Fatalf("launched %+v, want %+v", shell.launched, want)
	}
	if *dismissed != 2 {
		t.Fatalf("the search bar was closed %d times, want 2", *dismissed)
	}
}

func TestRevealShowsTheFileInExplorer(t *testing.T) {
	svc, shell, root, dismissed := setup(t)
	file := touch(t, filepath.Join(root, "a.txt"))
	if err := svc.Reveal(file); err != nil {
		t.Fatal(err)
	}
	if len(shell.launched) != 1 || shell.launched[0] != (launch{"reveal", file, 0}) || *dismissed != 1 {
		t.Fatalf("launched %+v, dismissed %d", shell.launched, *dismissed)
	}
}

func TestPathsOutsideTheIndexedFoldersAreRefused(t *testing.T) {
	svc, shell, root, dismissed := setup(t)
	outside := touch(t, filepath.Join(t.TempDir(), "secret.txt"))
	sneaky := filepath.Join(root, "..", filepath.Base(filepath.Dir(outside)), "secret.txt")
	tests := []struct{ name, path string }{
		{"another folder", outside},
		{"a dot-dot escape", sneaky},
		{"a relative path", "secret.txt"},
		{"an empty path", ""},
		{"a program", testpath.P(`C:\Windows\System32\cmd.exe`)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := svc.Open(tt.path, 0); err == nil || !strings.Contains(err.Error(), "not inside") {
				t.Fatalf("Open(%q) error = %v, want a refusal", tt.path, err)
			}
			if err := svc.Reveal(tt.path); err == nil {
				t.Fatalf("Reveal(%q) was allowed", tt.path)
			}
		})
	}
	if len(shell.launched) != 0 || *dismissed != 0 {
		t.Fatalf("something was launched or dismissed: %+v, %d", shell.launched, *dismissed)
	}
}

func TestAFileThatIsGoneIsReportedNotLaunched(t *testing.T) {
	svc, shell, root, _ := setup(t)
	err := svc.Open(filepath.Join(root, "deleted.txt"), 0)
	if err == nil || !strings.Contains(err.Error(), "no longer there") {
		t.Fatalf("error = %v", err)
	}
	if len(shell.launched) != 0 {
		t.Fatal("a missing file was launched")
	}
}

func TestInsideIsCaseInsensitiveAndRespectsFolderBoundaries(t *testing.T) {
	roots := []string{testpath.P(`C:\Users\me\Docs`)}
	tests := []struct {
		path string
		want bool
	}{
		{testpath.P(`C:\Users\me\Docs`), true},
		{testpath.P(`C:\Users\me\Docs\a.txt`), true},
		{testpath.P(`c:\users\ME\docs\Sub\b.txt`), true},
		{testpath.P(`C:\Users\me\Docs\..\Docs\a.txt`), true},
		{testpath.P(`C:\Users\me\Docs\..\Secrets\a.txt`), false},
		{testpath.P(`C:\Users\me\DocsExtra\a.txt`), false}, // shares a prefix, is another folder
		{testpath.P(`C:\Users\me`), false},
		{testpath.P(`D:\Users\me\Docs\a.txt`), false},
		{`Docs\a.txt`, false},
	}
	for _, tt := range tests {
		if got := inside(roots, tt.path); got != tt.want {
			t.Errorf("inside(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
	if inside(nil, testpath.P(`C:\a`)) {
		t.Error("with no roots nothing is inside")
	}
}

func TestSoundsAndVideosOpenInTheirDefaultPlayerNotAnEditor(t *testing.T) {
	for _, name := range []string{"a.mp3", "a.WAV", "a.flac", "a.m4a", "a.aac", "a.ogg", "a.opus", "a.wma", "a.mp4", "a.m4v", "a.mov", "a.mkv", "a.webm", "a.avi", "a.wmv"} {
		if !opensInDefaultApp(name) {
			t.Errorf("%s should open in the program that plays it, not in a text editor", name)
		}
	}
	svc, shell, root, _ := setup(t)
	clip := touch(t, filepath.Join(root, "clip.mp4"))
	if err := svc.Open(clip, 0); err != nil {
		t.Fatal(err)
	}
	if len(shell.launched) != 1 || shell.launched[0] != (launch{"default", clip, 0}) {
		t.Fatalf("launched %+v", shell.launched)
	}
}

func TestOpenAtPlaysAMomentInAPlayerThatCanBeAskedForIt(t *testing.T) {
	svc, shell, root, dismissed := setup(t)
	shell.player = &Player{Name: "VLC", Exe: testpath.P(`C:\vlc.exe`)}
	talk := touch(t, filepath.Join(root, "talks", "sleep.mp3"))
	if err := svc.OpenAt(talk, 75); err != nil {
		t.Fatal(err)
	}
	if len(shell.plays) != 1 || shell.plays[0] != (played{"VLC", talk, 75}) || len(shell.launched) != 0 || *dismissed != 1 {
		t.Fatalf("plays %+v, launched %+v, dismissed %d", shell.plays, shell.launched, *dismissed)
	}
	if got := svc.SeekPlayer(); got != "VLC" {
		t.Fatalf("SeekPlayer = %q", got)
	}
}

func TestOpenAtWithoutSuchAPlayerOpensTheFileFromTheStart(t *testing.T) {
	svc, shell, root, dismissed := setup(t)
	talk := touch(t, filepath.Join(root, "sleep.mp3"))
	if err := svc.OpenAt(talk, 75); err != nil {
		t.Fatal(err)
	}
	if len(shell.plays) != 0 || len(shell.launched) != 1 || shell.launched[0] != (launch{"default", talk, 0}) || *dismissed != 1 {
		t.Fatalf("plays %+v, launched %+v, dismissed %d", shell.plays, shell.launched, *dismissed)
	}
	if svc.SeekPlayer() != "" {
		t.Fatal("no player was installed, yet one was named")
	}
}

func TestOpenAtOnSomethingThatIsNotARecordingJustOpensIt(t *testing.T) {
	svc, shell, root, _ := setup(t)
	shell.player = &Player{Name: "VLC", Exe: testpath.P(`C:\vlc.exe`)}
	script := touch(t, filepath.Join(root, "deploy.js"))
	if err := svc.OpenAt(script, 30); err != nil {
		t.Fatal(err)
	}
	if len(shell.plays) != 0 || len(shell.launched) != 1 || shell.launched[0].how != "editor" {
		t.Fatalf("plays %+v, launched %+v: a script must never reach a player", shell.plays, shell.launched)
	}
}

func TestOpenAtKeepsTheRulesOfOpen(t *testing.T) {
	svc, shell, root, _ := setup(t)
	shell.player = &Player{Name: "VLC", Exe: testpath.P(`C:\vlc.exe`)}
	if err := svc.OpenAt(testpath.P(`C:\Windows\media\x.mp3`), 1); err == nil || !strings.Contains(err.Error(), "not inside") {
		t.Fatalf("a file outside the folders was played: %v", err)
	}
	if err := svc.OpenAt(filepath.Join(root, "gone.mp3"), 1); err == nil || !strings.Contains(err.Error(), "no longer there") {
		t.Fatalf("a deleted file was played: %v", err)
	}
	if len(shell.plays) != 0 {
		t.Fatalf("plays %+v", shell.plays)
	}
}
