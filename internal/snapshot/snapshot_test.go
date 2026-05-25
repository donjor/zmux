package snapshot

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/donjor/zmux/internal/terminal"
	"github.com/donjor/zmux/internal/tmux"
)

// --- in-memory FS (config.FS) ---

type memFS struct {
	files map[string][]byte
	home  string
}

func newMemFS() *memFS { return &memFS{files: map[string][]byte{}, home: "/home/u"} }

func (m *memFS) ReadFile(path string) ([]byte, error) {
	if d, ok := m.files[path]; ok {
		return d, nil
	}
	return nil, &os.PathError{Op: "open", Path: path, Err: os.ErrNotExist}
}

func (m *memFS) WriteFile(path string, data []byte, _ os.FileMode) error {
	m.files[path] = append([]byte{}, data...)
	return nil
}
func (m *memFS) MkdirAll(string, os.FileMode) error { return nil }
func (m *memFS) Stat(path string) (os.FileInfo, error) {
	if _, ok := m.files[path]; ok {
		return nil, nil
	}
	return nil, &os.PathError{Op: "stat", Path: path, Err: os.ErrNotExist}
}
func (m *memFS) UserHomeDir() (string, error)  { return m.home, nil }
func (m *memFS) Glob(string) ([]string, error) { return nil, nil }
func (m *memFS) has(path string) bool          { _, ok := m.files[path]; return ok }

// --- fakes ---

type fakeResolver struct {
	res terminal.Result
	err error
}

func (f fakeResolver) Resolve(context.Context) (terminal.Result, error) { return f.res, f.err }

type fakeShooter struct {
	fs      *memFS // when set, simulates the tool writing the PNG file
	err     error
	calls   int
	lastGeo string
}

func (f *fakeShooter) Shoot(geometry, outPath string) error {
	f.calls++
	f.lastGeo = geometry
	if f.err != nil {
		return f.err
	}
	if f.fs != nil {
		_ = f.fs.WriteFile(outPath, []byte("PNG"), 0o644)
	}
	return nil
}

func okResolver(geometry string) fakeResolver {
	return fakeResolver{res: terminal.Result{OK: true, Status: terminal.StatusOK, Target: &terminal.Target{Geometry: geometry, Visible: true}}}
}

func newSnapshotter(fs *memFS, runner tmux.Runner, r TargetResolver, sh Shooter) Snapshotter {
	return Snapshotter{
		Runner:   runner,
		FS:       fs,
		Resolver: r,
		Shooter:  sh,
		Now:      func() time.Time { return time.Date(2026, 5, 25, 8, 30, 0, 0, time.UTC) },
	}
}

// --- tests ---

func TestCaptureFullBundle(t *testing.T) {
	fs := newMemFS()
	runner := &tmux.MockRunner{
		InsideTmux:           true,
		DisplayMessageResult: "%5 120x40 active=1 title=vim command=nvim path=/tmp",
		CapturePaneOptsFunc: func(_ string, opts tmux.CapturePaneOptions) (string, error) {
			if opts.ANSI {
				return "ANSI-BODY", nil
			}
			return "TEXT-BODY", nil
		},
	}
	sh := &fakeShooter{fs: fs}
	s := newSnapshotter(fs, runner, okResolver("12,57 2536x1371"), sh)

	res, err := s.Capture(context.Background(), Options{
		Dir:   "/snap",
		Panes: []PaneTarget{{Name: "main", PaneID: "%5"}},
		Lines: 200,
		PNG:   true,
	})
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}

	if !res.OK {
		t.Errorf("OK = false, want true; warnings=%v", res.Warnings)
	}
	wantMod := map[string]bool{"tmux_text": true, "tmux_ansi": true, "screenshot_png": true}
	for _, m := range res.Modalities {
		delete(wantMod, m)
	}
	if len(wantMod) != 0 {
		t.Errorf("missing modalities: %v (got %v)", wantMod, res.Modalities)
	}
	if sh.calls != 1 || sh.lastGeo != "12,57 2536x1371" {
		t.Errorf("shooter calls=%d geo=%q", sh.calls, sh.lastGeo)
	}
	for _, f := range []string{
		"/snap/main.ansi", "/snap/main.txt", "/snap/main.meta.json",
		"/snap/terminal.png.meta.json", "/snap/snapshot.json", "/snap/manifest.json", "/snap/README.md",
	} {
		if !fs.has(f) {
			t.Errorf("missing artifact %q", f)
		}
	}
	if got := string(fs.files["/snap/main.ansi"]); got != "ANSI-BODY" {
		t.Errorf("ansi body = %q", got)
	}
	if p := res.Panes[0]; p.Width != 120 || p.Height != 40 {
		t.Errorf("pane size = %dx%d, want 120x40", p.Width, p.Height)
	}

	// snapshot.json is valid + round-trips the result shape.
	var rt Result
	if err := json.Unmarshal(fs.files["/snap/snapshot.json"], &rt); err != nil {
		t.Fatalf("snapshot.json invalid: %v", err)
	}
	if rt.SchemaVersion != SchemaVersion || rt.Type != Type {
		t.Errorf("schema/type = %q/%q", rt.SchemaVersion, rt.Type)
	}
}

func TestCaptureScreenshotRefused(t *testing.T) {
	fs := newMemFS()
	runner := &tmux.MockRunner{InsideTmux: true, CapturedPaneContent: "x"}
	sh := &fakeShooter{}
	resolver := fakeResolver{res: terminal.Result{OK: false, Status: terminal.StatusHidden, Reason: "window hidden"}}
	s := newSnapshotter(fs, runner, resolver, sh)

	res, _ := s.Capture(context.Background(), Options{Dir: "/snap", Panes: []PaneTarget{{Name: "p", PaneID: "%1"}}, PNG: true})

	if res.ScreenshotPath != "" {
		t.Errorf("ScreenshotPath = %q, want empty on refusal", res.ScreenshotPath)
	}
	if sh.calls != 0 {
		t.Errorf("shooter called %d times despite refusal", sh.calls)
	}
	if !fs.has("/snap/terminal.png.meta.json") {
		t.Error("expected refusal meta file")
	}
	if !containsSubstr(res.Warnings, "refused") {
		t.Errorf("expected refusal warning, got %v", res.Warnings)
	}
	for _, m := range res.Modalities {
		if m == "screenshot_png" {
			t.Error("screenshot_png modality must not be present on refusal")
		}
	}
}

func TestCaptureShooterFailure(t *testing.T) {
	fs := newMemFS()
	runner := &tmux.MockRunner{InsideTmux: true, CapturedPaneContent: "x"}
	sh := &fakeShooter{err: errors.New("grim boom")}
	s := newSnapshotter(fs, runner, okResolver("0,0 100x100"), sh)

	res, _ := s.Capture(context.Background(), Options{Dir: "/snap", Panes: []PaneTarget{{Name: "p", PaneID: "%1"}}, PNG: true})

	if res.ScreenshotPath != "" {
		t.Errorf("ScreenshotPath = %q, want empty on shoot failure", res.ScreenshotPath)
	}
	if !containsSubstr(res.Warnings, "screenshot tool failed") {
		t.Errorf("expected shoot-failure warning, got %v", res.Warnings)
	}
}

func TestCaptureShooterReportsSuccessButNoFile(t *testing.T) {
	fs := newMemFS()
	runner := &tmux.MockRunner{InsideTmux: true, CapturedPaneContent: "x"}
	sh := &fakeShooter{} // no fs → Shoot returns nil but writes nothing
	s := newSnapshotter(fs, runner, okResolver("0,0 100x100"), sh)

	res, _ := s.Capture(context.Background(), Options{Dir: "/snap", Panes: []PaneTarget{{Name: "p", PaneID: "%1"}}, PNG: true})

	if res.ScreenshotPath != "" {
		t.Errorf("ScreenshotPath = %q, want empty when no PNG file written", res.ScreenshotPath)
	}
	if !containsSubstr(res.Warnings, "no PNG file") {
		t.Errorf("expected missing-file warning, got %v", res.Warnings)
	}
}

func TestCaptureAnsiFailsTextSucceeds(t *testing.T) {
	fs := newMemFS()
	runner := &tmux.MockRunner{
		InsideTmux: true,
		CapturePaneOptsFunc: func(_ string, opts tmux.CapturePaneOptions) (string, error) {
			if opts.ANSI {
				return "", errors.New("no ANSI")
			}
			return "TEXT", nil
		},
	}
	s := newSnapshotter(fs, runner, nil, nil)

	res, _ := s.Capture(context.Background(), Options{Dir: "/snap", Panes: []PaneTarget{{Name: "p", PaneID: "%1"}}})

	if len(res.Panes) != 1 || res.Panes[0].AnsiPath != "" || res.Panes[0].TextPath == "" {
		t.Errorf("expected text-only pane, got %+v", res.Panes)
	}
	if !contains(res.Modalities, "tmux_text") || contains(res.Modalities, "tmux_ansi") {
		t.Errorf("modalities = %v, want text-only", res.Modalities)
	}
}

func TestCaptureBothCapturesFailSkipsPane(t *testing.T) {
	fs := newMemFS()
	runner := &tmux.MockRunner{
		InsideTmux:          true,
		CapturePaneOptsFunc: func(string, tmux.CapturePaneOptions) (string, error) { return "", errors.New("dead pane") },
	}
	s := newSnapshotter(fs, runner, nil, nil)

	res, _ := s.Capture(context.Background(), Options{Dir: "/snap", Panes: []PaneTarget{{Name: "p", PaneID: "%1"}}})

	if len(res.Panes) != 0 {
		t.Errorf("expected pane skipped, got %+v", res.Panes)
	}
	if res.OK {
		t.Error("OK should be false when nothing captured")
	}
	if fs.has("/snap/p.meta.json") {
		t.Error("no meta file should be written for fully-failed pane")
	}
}

func TestCaptureNoPanesWarns(t *testing.T) {
	fs := newMemFS()
	s := newSnapshotter(fs, &tmux.MockRunner{InsideTmux: true}, nil, nil)
	res, _ := s.Capture(context.Background(), Options{Dir: "/snap"})
	if res.OK {
		t.Error("OK should be false with no panes")
	}
	if !containsSubstr(res.Warnings, "no panes") {
		t.Errorf("expected no-panes warning, got %v", res.Warnings)
	}
}

func TestSeedWarningsPreserved(t *testing.T) {
	fs := newMemFS()
	s := newSnapshotter(fs, &tmux.MockRunner{InsideTmux: true, CapturedPaneContent: "x"}, nil, nil)
	res, _ := s.Capture(context.Background(), Options{
		Dir:      "/snap",
		Panes:    []PaneTarget{{Name: "p", PaneID: "%1"}},
		Warnings: []string{"PNG skipped: requested panes do not include the current terminal"},
	})
	if !containsSubstr(res.Warnings, "PNG skipped") {
		t.Errorf("seed warning lost: %v", res.Warnings)
	}
}

func TestClampLines(t *testing.T) {
	cases := map[int]int{0: 120, -5: 120, 1: 1, 50: 50, 2000: 2000, 9999: 2000}
	for in, want := range cases {
		if got := clampLines(in); got != want {
			t.Errorf("clampLines(%d) = %d, want %d", in, got, want)
		}
	}
}

func TestStamp(t *testing.T) {
	got := Stamp(time.Date(2026, 5, 25, 8, 30, 0, 0, time.UTC))
	if got != "2026-05-25T08-30-00-000Z" {
		t.Errorf("Stamp = %q", got)
	}
	if strings.ContainsAny(got, ":.") {
		t.Errorf("Stamp %q must be filesystem-safe", got)
	}
}

func TestPaneFileStem(t *testing.T) {
	cases := map[string]string{"main": "main", "Side Car!": "side-car", "  ": "pane", "%5": "5", "a/b": "a-b"}
	for in, want := range cases {
		if got := paneFileStem(in); got != want {
			t.Errorf("paneFileStem(%q) = %q, want %q", in, got, want)
		}
	}
	if long := paneFileStem(strings.Repeat("x", 200)); len(long) > maxStemLen {
		t.Errorf("paneFileStem long = %d chars, want <= %d", len(long), maxStemLen)
	}
}

func TestParsePaneSize(t *testing.T) {
	w, h := parsePaneSize("%5 120x40 active=1")
	if w != 120 || h != 40 {
		t.Errorf("parsePaneSize = %dx%d, want 120x40", w, h)
	}
	if w, h := parsePaneSize("no size here"); w != 0 || h != 0 {
		t.Errorf("parsePaneSize(no match) = %dx%d, want 0x0", w, h)
	}
}

// --- helpers ---

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

func containsSubstr(ss []string, sub string) bool {
	for _, x := range ss {
		if strings.Contains(x, sub) {
			return true
		}
	}
	return false
}
