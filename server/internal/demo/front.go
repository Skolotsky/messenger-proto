package demo

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Front is the one address people use: the product at /, the control panel at
// /control/, the notification app at /notifications/, the operator calls at
// /ops/<part>/ and the door at /orgs/. Without webDir it serves only the
// operations, as a part does.
func (s *Service) Front(webDir string) http.Handler {
	api := http.NewServeMux()
	s.Routes(api)

	parts := map[string]http.Handler{
		"conversations": proxy(s.cfg.Conversations),
		"delivery":      proxy(s.cfg.Delivery),
		"search":        proxy(s.cfg.Search),
		"sender":        proxy(s.cfg.Sender),
		"feed":          proxy(s.cfg.Feed),
		"demo":          api,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/ops/{part}/", func(w http.ResponseWriter, r *http.Request) {
		h, ok := parts[r.PathValue("part")]
		if !ok {
			answer(w, http.StatusNotFound, map[string]string{"refused": "no such part"})
			return
		}
		stripped(h, "/ops/"+r.PathValue("part")).ServeHTTP(w, r)
	})
	mux.Handle("/demo/", stripped(api, "/demo"))
	s.Routes(mux)
	if webDir == "" {
		return mux
	}
	mux.Handle("/orgs/", parts["conversations"])
	mux.Handle("/control/", http.StripPrefix("/control", pages(filepath.Join(webDir, "control"))))
	mux.Handle("/notifications/", http.StripPrefix("/notifications", pages(filepath.Join(webDir, "notifications"))))
	app := pages(filepath.Join(webDir, "app"))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" && r.URL.Query().Get("org") == "" {
			s.entry(w, r)
			return
		}
		app.ServeHTTP(w, r)
	})
	return mux
}

// entry sends a screen to the current organization (D-033).
func (s *Service) entry(w http.ResponseWriter, r *http.Request) {
	cur, err := s.current(r.Context())
	if err != nil || cur == nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Retry-After", "2")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`<!doctype html><meta charset="utf-8"><meta http-equiv="refresh" content="2"><title>Contexia</title>` +
			`<p style="font-family:system-ui;margin:2rem">The team is being set up. This page opens by itself when it is ready.</p>`))
		return
	}
	q := r.URL.Query()
	q.Set("org", cur.Org)
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, "/?"+q.Encode(), http.StatusFound)
}

// stripped removes the prefix from both forms of the path, so an encoded
// conversation name such as %23launch reaches the part as it was sent.
func stripped(h http.Handler, prefix string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c := r.Clone(r.Context())
		c.URL.Path = strings.TrimPrefix(r.URL.Path, prefix)
		if r.URL.RawPath != "" {
			c.URL.RawPath = strings.TrimPrefix(r.URL.RawPath, prefix)
		}
		h.ServeHTTP(w, c)
	})
}

// proxy passes a call on, including a live connection's upgrade. An unreachable
// part answers 502 at once, which the pages show as that part being down.
func proxy(base string) http.Handler {
	target, err := url.Parse(base)
	if err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			answer(w, http.StatusBadGateway, map[string]string{"not_available": "bad address"})
		})
	}
	p := httputil.NewSingleHostReverseProxy(target)
	p.FlushInterval = -1
	p.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, _ error) {
		answer(w, http.StatusBadGateway, map[string]string{"not_available": "that part is not answering"})
	}
	return p
}

// pages serves a built single-page app: a file if there is one, else its index.
func pages(root string) http.Handler {
	files := http.FileServer(http.Dir(root))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := path.Clean("/" + r.URL.Path)
		if info, err := os.Stat(filepath.Join(root, filepath.FromSlash(name))); err != nil || info.IsDir() {
			w.Header().Set("Cache-Control", "no-cache")
			http.ServeFile(w, r, filepath.Join(root, "index.html"))
			return
		}
		files.ServeHTTP(w, r)
	})
}
