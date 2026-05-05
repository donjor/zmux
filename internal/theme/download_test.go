package theme

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

// mockHTTPClient implements HTTPClient for testing.
type mockHTTPClient struct {
	resp *http.Response
	err  error
}

func (m *mockHTTPClient) Get(url string) (*http.Response, error) {
	return m.resp, m.err
}

// buildTarGz creates a tar.gz archive in memory with the given file entries.
// entries is a map from path (inside the tarball) to content.
func buildTarGz(entries map[string]string) *bytes.Buffer {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	for name, content := range entries {
		hdr := &tar.Header{
			Name: name,
			Mode: 0o644,
			Size: int64(len(content)),
		}
		_ = tw.WriteHeader(hdr)
		_, _ = tw.Write([]byte(content))
	}

	tw.Close()
	gw.Close()
	return &buf
}

func TestDownload_Success(t *testing.T) {
	// Build a fake tarball with two ghostty themes and one non-ghostty file.
	entries := map[string]string{
		"iTerm2-Color-Schemes-master/Ghostty/Dracula": "background = #282a36\nforeground = #f8f8f2\n",
		"iTerm2-Color-Schemes-master/Ghostty/Nord":    "background = #2e3440\nforeground = #d8dee9\n",
		"iTerm2-Color-Schemes-master/other/README.md": "# iterm2 color schemes\n",
	}
	tarball := buildTarGz(entries)

	client := &mockHTTPClient{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(tarball),
		},
	}

	destDir := t.TempDir()
	count, err := Download(client, destDir)
	if err != nil {
		t.Fatalf("Download error: %v", err)
	}

	if count != 2 {
		t.Errorf("Download count = %d, want 2", count)
	}

	// Check that the files were extracted.
	for _, name := range []string{"Dracula", "Nord"} {
		path := filepath.Join(destDir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("expected file %q to exist: %v", name, err)
			continue
		}
		if len(data) == 0 {
			t.Errorf("file %q is empty", name)
		}
	}

	// Check that non-Ghostty files were not extracted.
	path := filepath.Join(destDir, "README.md")
	if _, err := os.Stat(path); err == nil {
		t.Error("README.md should not have been extracted")
	}
}

func TestDownload_HTTPError(t *testing.T) {
	client := &mockHTTPClient{
		resp: &http.Response{
			StatusCode: http.StatusNotFound,
			Body:       io.NopCloser(bytes.NewReader(nil)),
		},
	}

	destDir := t.TempDir()
	_, err := Download(client, destDir)
	if err == nil {
		t.Fatal("expected error for HTTP 404")
	}
}

func TestDownload_NetworkError(t *testing.T) {
	client := &mockHTTPClient{
		err: fmt.Errorf("network error"),
	}

	destDir := t.TempDir()
	_, err := Download(client, destDir)
	if err == nil {
		t.Fatal("expected error for network failure")
	}
}

func TestDownload_EmptyTarball(t *testing.T) {
	entries := map[string]string{}
	tarball := buildTarGz(entries)

	client := &mockHTTPClient{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(tarball),
		},
	}

	destDir := t.TempDir()
	count, err := Download(client, destDir)
	if err != nil {
		t.Fatalf("Download error: %v", err)
	}
	if count != 0 {
		t.Errorf("Download count = %d, want 0", count)
	}
}

func TestDownload_SkipsSubdirectories(t *testing.T) {
	entries := map[string]string{
		"iTerm2-Color-Schemes-master/Ghostty/Theme1":     "bg = #000000\n",
		"iTerm2-Color-Schemes-master/Ghostty/sub/Theme2": "bg = #111111\n",
	}
	tarball := buildTarGz(entries)

	client := &mockHTTPClient{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(tarball),
		},
	}

	destDir := t.TempDir()
	count, err := Download(client, destDir)
	if err != nil {
		t.Fatalf("Download error: %v", err)
	}

	// Only direct children of Ghostty/ should be extracted.
	if count != 1 {
		t.Errorf("Download count = %d, want 1 (should skip subdirectories)", count)
	}
}
