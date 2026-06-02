package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

var storage_path = "data"
var port = "3333"

var shaRe = regexp.MustCompile(`^[0-9a-f]{4,40}$`)
var idUnsafeRe = regexp.MustCompile(`[^a-z0-9_-]+`)
var idDashesRe = regexp.MustCompile(`-+`)

// normalizeID lowercases, replaces any non [a-z0-9_-] run with a single dash,
// collapses repeated dashes, trims leading/trailing dashes, and caps length.
// Returns "" if nothing usable remains.
func normalizeID(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = idUnsafeRe.ReplaceAllString(s, "-")
	s = idDashesRe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 128 {
		s = s[:128]
	}
	return s
}

var gitRemote string

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func parseFlags(args []string) error {
	fs := flag.NewFlagSet("notesd", flag.ContinueOnError)
	fs.StringVar(&storage_path, "storage", envOr("STORAGE_PATH", "data"), "path to the git-backed storage directory (env: STORAGE_PATH)")
	fs.StringVar(&port, "port", envOr("PORT", "3333"), "TCP port to listen on (env: PORT)")
	fs.StringVar(&gitRemote, "git-remote", os.Getenv("GIT_REMOTE"), "optional git remote URL to push/pull on writes (env: GIT_REMOTE)")
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), `notesd - a tiny git-backed paste/notes server

Usage:
  notesd [flags]

Flags:
`)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if gitRemote != "" {
		os.Setenv("GIT_REMOTE", gitRemote)
	}
	return nil
}

func init() {
	storage_path = envOr("STORAGE_PATH", "data")
	port = envOr("PORT", "3333")
}

func git(args ...string) (string, error) {
	return gitIn(nil, args...)
}

func gitIn(stdin io.Reader, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", storage_path}, args...)...)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if stdin != nil {
		cmd.Stdin = stdin
	}
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %v: %s", strings.Join(args, " "), err, errb.String())
	}
	return out.String(), nil
}

func ensureRepo() error {
	if err := os.MkdirAll(storage_path, 0755); err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(storage_path, ".git")); err != nil {
		if _, err := git("init", "-q", "-b", "main"); err != nil {
			return err
		}
		if _, err := git("config", "user.email", "notesd@localhost"); err != nil {
			return err
		}
		if _, err := git("config", "user.name", "notesd"); err != nil {
			return err
		}
	}
	if remote := os.Getenv("GIT_REMOTE"); remote != "" {
		if _, err := git("remote", "set-url", "origin", remote); err != nil {
			if _, err := git("remote", "add", "origin", remote); err != nil {
				return err
			}
		}
	}
	return nil
}

var gitMu sync.Mutex

func pushAsync() {
	if os.Getenv("GIT_REMOTE") == "" {
		return
	}
	go func() {
		gitMu.Lock()
		defer gitMu.Unlock()
		if out, err := git("push", "-q", "origin", "HEAD"); err != nil {
			log.Printf("git push: %v %s", err, out)
		}
	}()
}

func pullSync() {
	if os.Getenv("GIT_REMOTE") == "" {
		return
	}
	gitMu.Lock()
	defer gitMu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-C", storage_path, "pull", "--ff-only", "-q", "origin", "HEAD")
	var errb bytes.Buffer
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			log.Printf("git pull: timeout after 5s")
		} else {
			log.Printf("git pull: %v %s", err, strings.TrimSpace(errb.String()))
		}
	}
}

func pullMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		pullSync()
		next.ServeHTTP(w, req)
	})
}

func headSHA() string {
	out, err := git("rev-parse", "HEAD")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

func preview(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	buf := make([]byte, 512)
	n, _ := f.Read(buf)
	return previewString(string(buf[:n]))
}

func previewString(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	out := make([]rune, 0, 80)
	for _, r := range s {
		if r == '\t' {
			r = ' '
		}
		if r < 0x20 || r == 0x7f {
			continue
		}
		out = append(out, r)
		if len(out) >= 80 {
			break
		}
	}
	return string(out)
}

func writeAndCommit(id string, body io.Reader, msg string) (string, error) {
	path := filepath.Join(storage_path, id)
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(f, body); err != nil {
		f.Close()
		return "", err
	}
	f.Close()
	if _, err := git("add", "--", id); err != nil {
		return "", err
	}
	if _, err := git("commit", "-q", "-m", msg, "--allow-empty"); err != nil {
		return "", err
	}
	pushAsync()
	return headSHA(), nil
}

func main() {
	if err := parseFlags(os.Args[1:]); err != nil {
		os.Exit(2)
	}
	if err := ensureRepo(); err != nil {
		panic(err)
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(pullMiddleware)

	write := func(w http.ResponseWriter, req *http.Request) {
		id := normalizeID(chi.URLParam(req, "id"))
		if id == "" {
			http.Error(w, "bad id", http.StatusBadRequest)
			return
		}
		defer req.Body.Close()
		sha, err := writeAndCommit(id, req.Body, "write "+id)
		if err != nil {
			log.Printf("write %s: %v", id, err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("X-Commit", sha)
		fmt.Fprintf(w, "%s", id)
	}
	r.Post("/{id}", write)
	r.Put("/{id}", write)

	r.Post("/", func(w http.ResponseWriter, req *http.Request) {
		var rnd [3]byte
		rand.Read(rnd[:])
		id := fmt.Sprintf("%d-%s", time.Now().Unix(), hex.EncodeToString(rnd[:]))
		defer req.Body.Close()
		sha, err := writeAndCommit(id, req.Body, "write "+id)
		if err != nil {
			log.Printf("write %s: %v", id, err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("X-Commit", sha)
		fmt.Fprintf(w, "%s", id)
	})

	r.Get("/{id}", func(w http.ResponseWriter, req *http.Request) {
		id := normalizeID(chi.URLParam(req, "id"))
		if id == "" {
			http.Error(w, "bad id", http.StatusBadRequest)
			return
		}
		version := req.URL.Query().Get("version")
		var content string
		if version != "" {
			if !shaRe.MatchString(version) {
				http.Error(w, "bad version", http.StatusBadRequest)
				return
			}
			out, err := git("show", version+":"+id)
			if err != nil {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			content = out
		} else {
			b, err := os.ReadFile(filepath.Join(storage_path, id))
			if err != nil {
				if wantsHTML(req) {
					// new paste: render empty editor
					w.Header().Set("Content-Type", "text/html; charset=utf-8")
					viewTmpl.Execute(w, map[string]any{"ID": id, "Content": "", "Version": ""})
					return
				}
				http.Error(w, "File not found", http.StatusNotFound)
				return
			}
			content = string(b)
		}
		if wantsHTML(req) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			viewTmpl.Execute(w, map[string]any{"ID": id, "Content": content, "Version": version})
			return
		}
		io.WriteString(w, content)
	})

	r.Get("/{id}/history", func(w http.ResponseWriter, req *http.Request) {
		id := normalizeID(chi.URLParam(req, "id"))
		if id == "" {
			http.Error(w, "bad id", http.StatusBadRequest)
			return
		}
		out, err := git("log", "--pretty=format:%H %ct", "--", id)
		if err != nil || strings.TrimSpace(out) == "" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		type hentry struct {
			SHA, Short, Unix, When, Preview string
		}
		var entries []hentry
		for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
			parts := strings.SplitN(line, " ", 2)
			if len(parts) != 2 {
				continue
			}
			sha := parts[0]
			content, err := git("show", sha+":"+id)
			p := "(deleted)"
			if err == nil {
				p = previewString(content)
			}
			when := parts[1]
			if secs, perr := strconv.ParseInt(parts[1], 10, 64); perr == nil {
				when = time.Unix(secs, 0).UTC().Format("2006-01-02 15:04:05")
			}
			entries = append(entries, hentry{SHA: sha, Short: sha[:8], Unix: parts[1], When: when, Preview: p})
		}
		if wantsHTML(req) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			historyTmpl.Execute(w, map[string]any{"ID": id, "Entries": entries})
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		for _, e := range entries {
			fmt.Fprintf(w, "%s %s %s\n", e.SHA, e.Unix, e.Preview)
		}
	})

	r.Get("/", func(w http.ResponseWriter, req *http.Request) {
		files, err := os.ReadDir(storage_path)
		if err != nil {
			log.Printf("readdir %s: %v", storage_path, err)
			http.Error(w, "Could not read directory", http.StatusInternalServerError)
			return
		}
		type entry struct {
			ID      string `json:"id"`
			Preview string `json:"preview"`
		}
		entries := []entry{}
		for _, file := range files {
			if file.IsDir() || strings.HasPrefix(file.Name(), ".") {
				continue
			}
			entries = append(entries, entry{
				ID:      file.Name(),
				Preview: preview(filepath.Join(storage_path, file.Name())),
			})
		}
		accept := req.Header.Get("Accept")
		if strings.Contains(accept, "application/json") {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(entries)
			return
		}
		if wantsHTML(req) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			indexTmpl.Execute(w, entries)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		for _, e := range entries {
			fmt.Fprintf(w, "%s\t%s\n", e.ID, e.Preview)
		}
	})

	r.Delete("/{id}", func(w http.ResponseWriter, req *http.Request) {
		id := normalizeID(chi.URLParam(req, "id"))
		if id == "" {
			http.Error(w, "bad id", http.StatusBadRequest)
			return
		}
		path := filepath.Join(storage_path, id)
		if _, err := os.Stat(path); err != nil {
			http.Error(w, "File not found", http.StatusNotFound)
			return
		}
		if _, err := git("rm", "-q", "--", id); err != nil {
			log.Printf("delete %s (rm): %v", id, err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if _, err := git("commit", "-q", "-m", "delete "+id); err != nil {
			log.Printf("delete %s (commit): %v", id, err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		pushAsync()
		w.Header().Set("X-Commit", headSHA())
		w.Write([]byte("File deleted successfully"))
	})

	log.Printf("serving on :%s\n", port)
	http.ListenAndServe(":"+port, r)
}
