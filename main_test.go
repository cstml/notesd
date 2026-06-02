package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestNormalizeID(t *testing.T) {
	cases := map[string]string{
		"  Hello World  ":     "hello-world",
		"foo//bar":            "foo-bar",
		"--a--b--":            "a-b",
		"":                    "",
		"!!!":                 "",
		"Already-Safe_id-9":   "already-safe_id-9",
		strings.Repeat("a", 200): strings.Repeat("a", 128),
	}
	for in, want := range cases {
		if got := normalizeID(in); got != want {
			t.Errorf("normalizeID(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPreviewString(t *testing.T) {
	cases := map[string]string{
		"hello\nworld":    "hello",
		"a\tb":            "a b",
		"with\x00null":    "withnull",
		strings.Repeat("x", 100): strings.Repeat("x", 80),
	}
	for in, want := range cases {
		if got := previewString(in); got != want {
			t.Errorf("previewString(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestWantsHTML(t *testing.T) {
	mk := func(a string) *http.Request {
		r := httptest.NewRequest("GET", "/", nil)
		r.Header.Set("Accept", a)
		return r
	}
	if !wantsHTML(mk("text/html")) {
		t.Error("want true for text/html")
	}
	if wantsHTML(mk("application/json,text/html")) {
		t.Error("json should win")
	}
	if wantsHTML(mk("*/*")) {
		t.Error("want false for */*")
	}
}

// setupRepo creates a fresh repo dir and points storage_path at it. Returns cleanup.
func setupRepo(t *testing.T) func() {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	prev := storage_path
	storage_path = dir
	if err := ensureRepo(); err != nil {
		t.Fatalf("ensureRepo: %v", err)
	}
	return func() { storage_path = prev }
}

func newRouter() http.Handler {
	r := chi.NewRouter()
	write := func(w http.ResponseWriter, req *http.Request) {
		id := normalizeID(chi.URLParam(req, "id"))
		if id == "" {
			http.Error(w, "bad id", http.StatusBadRequest)
			return
		}
		defer req.Body.Close()
		sha, err := writeAndCommit(id, req.Body, "write "+id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("X-Commit", sha)
		io.WriteString(w, id)
	}
	r.Post("/{id}", write)
	r.Put("/{id}", write)
	r.Get("/{id}", func(w http.ResponseWriter, req *http.Request) {
		id := normalizeID(chi.URLParam(req, "id"))
		b, err := os.ReadFile(filepath.Join(storage_path, id))
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Write(b)
	})
	r.Delete("/{id}", func(w http.ResponseWriter, req *http.Request) {
		id := normalizeID(chi.URLParam(req, "id"))
		path := filepath.Join(storage_path, id)
		if _, err := os.Stat(path); err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if _, err := git("rm", "-q", "--", id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if _, err := git("commit", "-q", "-m", "delete "+id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Write([]byte("ok"))
	})
	return r
}

func TestWriteReadDelete(t *testing.T) {
	defer setupRepo(t)()
	r := newRouter()

	// write
	req := httptest.NewRequest("POST", "/note1", strings.NewReader("hello"))
	req = req.WithContext(req.Context())
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("write: %d %s", rr.Code, rr.Body)
	}
	if rr.Header().Get("X-Commit") == "" {
		t.Error("missing X-Commit")
	}

	// read
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest("GET", "/note1", nil))
	if rr.Code != 200 || rr.Body.String() != "hello" {
		t.Fatalf("read: %d %q", rr.Code, rr.Body.String())
	}

	// delete
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest("DELETE", "/note1", nil))
	if rr.Code != 200 {
		t.Fatalf("delete: %d %s", rr.Code, rr.Body)
	}

	// read after delete
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest("GET", "/note1", nil))
	if rr.Code != 404 {
		t.Fatalf("want 404 after delete, got %d", rr.Code)
	}
}

func TestBadID(t *testing.T) {
	defer setupRepo(t)()
	r := newRouter()
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest("POST", "/!!!", strings.NewReader("x")))
	if rr.Code != 400 {
		t.Fatalf("want 400, got %d", rr.Code)
	}
}
