package enginesetup

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func artifactOf(url string, data []byte) Artifact {
	sum := sha256.Sum256(data)
	return Artifact{Name: "test file", URL: url, Size: int64(len(data)), SHA256: hex.EncodeToString(sum[:])}
}

// fileServer serves named files, with ranges, and records what it was asked for.
type fileServer struct {
	*httptest.Server
	mu      sync.Mutex
	files   map[string][]byte
	asked   []string // "path range"
	cutAt   int      // when positive, the next response ends after this many bytes (a dropped connection)
	noRange bool     // ignore Range and send the whole file with 200
	block   chan struct{} // when set, requests for blockPath wait for it to close (or for the client to give up)
	blockPath string
}

func newFileServer(t *testing.T, files map[string][]byte) *fileServer {
	t.Helper()
	s := &fileServer{files: files}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		data, ok := s.files[r.URL.Path]
		cut, noRange, block, blockPath := s.cutAt, s.noRange, s.block, s.blockPath
		s.asked = append(s.asked, r.URL.Path+" "+r.Header.Get("Range"))
		if cut > 0 {
			s.cutAt = 0
		}
		s.mu.Unlock()
		if !ok {
			http.NotFound(w, r)
			return
		}
		if block != nil && r.URL.Path == blockPath {
			select {
			case <-block:
			case <-r.Context().Done():
				return
			}
		}
		if cut > 0 {
			w.Header().Set("Content-Length", "1000000000")
			_, _ = w.Write(data[:min(cut, len(data))])
			w.(http.Flusher).Flush()
			panic(http.ErrAbortHandler) // drop the connection mid-body
		}
		if noRange {
			r.Header.Del("Range")
		}
		http.ServeContent(w, r, "f", time.Time{}, bytes.NewReader(data))
	}))
	t.Cleanup(s.Close)
	return s
}

func (s *fileServer) requests() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.asked...)
}

func payload(n int, seed byte) []byte {
	data := make([]byte, n)
	for i := range data {
		data[i] = byte(i*7) + seed
	}
	return data
}

func TestADownloadIsRenamedOnlyOnceItsSizeAndChecksumMatch(t *testing.T) {
	data := payload(600_000, 1)
	srv := newFileServer(t, map[string][]byte{"/a.bin": data})
	art := artifactOf(srv.URL+"/a.bin", data)
	dest := filepath.Join(t.TempDir(), "models", "a.bin")
	var last int64
	if err := (Downloader{}).Download(t.Context(), art, dest, func(done, total int64) {
		if total != art.Size || done < last {
			t.Errorf("progress went from %d to %d of %d", last, done, total)
		}
		last = done
	}); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(dest)
	if !bytes.Equal(got, data) || last != art.Size {
		t.Fatalf("file differs or progress ended at %d", last)
	}
	if _, err := os.Stat(dest + ".part"); !os.IsNotExist(err) {
		t.Fatalf("the part file was left behind: %v", err)
	}
}

func TestACutDownloadContinuesWhereItStopped(t *testing.T) {
	data := payload(900_000, 2)
	srv := newFileServer(t, map[string][]byte{"/a.bin": data})
	art := artifactOf(srv.URL+"/a.bin", data)
	dest := filepath.Join(t.TempDir(), "a.bin")
	srv.cutAt = 300_000
	if err := (Downloader{}).Download(t.Context(), art, dest, nil); err == nil {
		t.Fatal("a connection that dropped mid-body was reported as a success")
	}
	have := fileSize(dest + ".part")
	if have == 0 || have >= art.Size {
		t.Fatalf("the part holds %d bytes after the cut", have)
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatal("a half file sits under the real name")
	}
	if err := (Downloader{}).Download(t.Context(), art, dest, nil); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(dest); !bytes.Equal(got, data) {
		t.Fatal("the resumed file differs from the original")
	}
	asked := srv.requests()
	if want := "/a.bin bytes=" + strconv.FormatInt(have, 10) + "-"; asked[len(asked)-1] != want {
		t.Fatalf("the second request was %q, want %q", asked[len(asked)-1], want)
	}
}

func TestAServerThatIgnoresRangesGivesTheWholeFileAndItIsStillRight(t *testing.T) {
	data := payload(400_000, 3)
	srv := newFileServer(t, map[string][]byte{"/a.bin": data})
	art := artifactOf(srv.URL+"/a.bin", data)
	dest := filepath.Join(t.TempDir(), "a.bin")
	if err := os.WriteFile(dest+".part", data[:100_000], 0o644); err != nil {
		t.Fatal(err)
	}
	srv.noRange = true
	if err := (Downloader{}).Download(t.Context(), art, dest, nil); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(dest); !bytes.Equal(got, data) {
		t.Fatal("restarting from zero left a wrong file")
	}
}

func TestAWrongFileIsRefusedAndDeleted(t *testing.T) {
	good := payload(300_000, 4)
	bad := payload(300_000, 9) // the same size, other content
	srv := newFileServer(t, map[string][]byte{"/a.bin": bad})
	art := artifactOf(srv.URL+"/a.bin", good)
	dest := filepath.Join(t.TempDir(), "a.bin")
	err := (Downloader{}).Download(t.Context(), art, dest, nil)
	if err == nil || !strings.Contains(err.Error(), "not the one that was expected") {
		t.Fatalf("Download error = %v", err)
	}
	for _, path := range []string{dest, dest + ".part"} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("%s was kept", path)
		}
	}
}

func TestAnErrorFromTheServerIsAnError(t *testing.T) {
	srv := newFileServer(t, map[string][]byte{})
	art := artifactOf(srv.URL+"/missing.bin", payload(10, 1))
	err := (Downloader{}).Download(t.Context(), art, filepath.Join(t.TempDir(), "a.bin"), nil)
	if err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("Download error = %v", err)
	}
}

func TestAFileThatIsAlreadyThereAndRightIsNotFetchedAgain(t *testing.T) {
	data := payload(200_000, 5)
	srv := newFileServer(t, map[string][]byte{"/a.bin": data})
	art := artifactOf(srv.URL+"/a.bin", data)
	dest := filepath.Join(t.TempDir(), "a.bin")
	if err := os.WriteFile(dest, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := (Downloader{}).Download(t.Context(), art, dest, nil); err != nil {
		t.Fatal(err)
	}
	if n := len(srv.requests()); n != 0 {
		t.Fatalf("%d requests for a file that was already there", n)
	}
}

func TestAPartLargerThanTheFileIsStartedOver(t *testing.T) {
	data := payload(100_000, 6)
	srv := newFileServer(t, map[string][]byte{"/a.bin": data})
	art := artifactOf(srv.URL+"/a.bin", data)
	dest := filepath.Join(t.TempDir(), "a.bin")
	if err := os.WriteFile(dest+".part", payload(250_000, 6), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := (Downloader{}).Download(t.Context(), art, dest, nil); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(dest); !bytes.Equal(got, data) {
		t.Fatal("the file differs")
	}
}

func makeZip(t *testing.T, files map[string]string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "x.zip")
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, body := range files {
		f, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = f.Write([]byte(body))
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestUnzipWritesTheFilesAndRefusesPathsThatLeaveTheFolder(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "out")
	if err := Unzip(makeZip(t, map[string]string{"llama-server.exe": "x", "sub/a.dll": "y"}), dir); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"llama-server.exe", filepath.Join("sub", "a.dll")} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("%s was not extracted: %v", name, err)
		}
	}
	evil := makeZip(t, map[string]string{"../evil.txt": "no"})
	err := Unzip(evil, dir)
	if err == nil || !strings.Contains(err.Error(), "outside the folder") {
		t.Fatalf("Unzip error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(dir), "evil.txt")); !os.IsNotExist(err) {
		t.Fatal("a file was written outside the folder")
	}
}
