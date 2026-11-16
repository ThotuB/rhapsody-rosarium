package windows

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestIntroVideoServer(t *testing.T) {
	video := filepath.Join(t.TempDir(), "intro.mp4")
	data := []byte(strings.Repeat("0123456789", 1000)) // 10 kB stands in for the video
	if err := os.WriteFile(video, data, 0o644); err != nil {
		t.Fatal(err)
	}

	url := ServeIntroVideo(video)
	if runtime.GOOS != "linux" {
		if url != IntroPath {
			t.Fatalf("off Linux the app's own path: %q", url)
		}
		return
	}
	if !strings.HasPrefix(url, "http://127.0.0.1:") || !strings.HasSuffix(url, IntroPath) {
		t.Fatalf("on Linux a loopback url: %q", url)
	}

	// what WebKit's video player does: ask for part of the file
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Range", "bytes=100-199")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusPartialContent || res.Header.Get("Content-Range") != "bytes 100-199/10000" || string(body) != string(data[100:200]) {
		t.Errorf("range request: %d %q, %d bytes", res.StatusCode, res.Header.Get("Content-Range"), len(body))
	}
	if ct := res.Header.Get("Content-Type"); ct != "video/mp4" {
		t.Errorf("content type %q, want video/mp4", ct)
	}

	other, err := http.Get(strings.TrimSuffix(url, IntroPath) + "/museum-bulbs.json")
	if err != nil {
		t.Fatal(err)
	}
	other.Body.Close()
	if other.StatusCode != http.StatusNotFound {
		t.Errorf("only the video is served, not other files: %d", other.StatusCode)
	}
}
