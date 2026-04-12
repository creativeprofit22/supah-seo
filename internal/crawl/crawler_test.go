package crawl

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/supah-seo/supah-seo/internal/provider/local"
)

func newTestSite() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			w.WriteHeader(404)
			_, _ = w.Write([]byte("not found"))
			return
		}
		_, _ = w.Write([]byte(`<html><head><title>Home</title></head><body>
			<h1>Welcome</h1>
			<a href="/about">About</a>
			<a href="/blog">Blog</a>
		</body></html>`))
	})
	mux.HandleFunc("/about", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html><head><title>About</title></head><body>
			<h1>About Us</h1>
			<a href="/">Home</a>
			<a href="/contact">Contact</a>
		</body></html>`))
	})
	mux.HandleFunc("/blog", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html><head><title>Blog</title></head><body>
			<h1>Blog</h1>
			<a href="/blog/post1">Post 1</a>
		</body></html>`))
	})
	mux.HandleFunc("/contact", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html><head><title>Contact</title></head><body>
			<h1>Contact</h1>
		</body></html>`))
	})
	mux.HandleFunc("/blog/post1", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html><head><title>Post 1</title></head><body>
			<h1>Post 1</h1>
			<a href="/blog/post2">Post 2</a>
		</body></html>`))
	})
	mux.HandleFunc("/blog/post2", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html><head><title>Post 2</title></head><body>
			<h1>Post 2</h1>
		</body></html>`))
	})
	return httptest.NewServer(mux)
}

func TestCrawlerDepthLimit(t *testing.T) {
	srv := newTestSite()
	defer srv.Close()

	fetcher := local.New()
	svc := NewService(fetcher)

	result, err := svc.Run(context.Background(), Request{
		TargetURL: srv.URL,
		Depth:     1,
		MaxPages:  50,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Depth 1: should get /, /about, /blog (linked from /) but not deeper
	if len(result.Pages) < 1 {
		t.Fatal("expected at least 1 page")
	}

	// Should not have depth-2 pages like /contact, /blog/post1
	for _, p := range result.Pages {
		if p.Title == "Post 1" || p.Title == "Post 2" {
			t.Errorf("should not have crawled depth-2 page: %s", p.URL)
		}
	}
}

func TestCrawlerMaxPages(t *testing.T) {
	srv := newTestSite()
	defer srv.Close()

	fetcher := local.New()
	svc := NewService(fetcher)

	result, err := svc.Run(context.Background(), Request{
		TargetURL: srv.URL,
		Depth:     10,
		MaxPages:  2,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Pages) > 2 {
		t.Errorf("expected at most 2 pages, got %d", len(result.Pages))
	}
}

func TestCrawlerSameDomainOnly(t *testing.T) {
	external := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<html><head><title>External</title></head></html>"))
	}))
	defer external.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, `<html><head><title>Home</title></head><body>
			<a href="%s">External</a>
		</body></html>`, external.URL)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	fetcher := local.New()
	svc := NewService(fetcher)

	result, err := svc.Run(context.Background(), Request{
		TargetURL: srv.URL,
		Depth:     3,
		MaxPages:  50,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, p := range result.Pages {
		if p.Title == "External" {
			t.Error("should not have crawled external domain")
		}
	}
}

func TestCrawlerHandlesErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			_, _ = w.Write([]byte(`<html><body><a href="/broken">Broken</a></body></html>`))
			return
		}
		w.WriteHeader(500)
		_, _ = w.Write([]byte("server error"))
	}))
	defer srv.Close()

	fetcher := local.New()
	svc := NewService(fetcher)

	result, err := svc.Run(context.Background(), Request{
		TargetURL: srv.URL,
		Depth:     2,
		MaxPages:  50,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have crawled both pages (500 is still a valid response)
	if len(result.Pages) < 2 {
		t.Errorf("expected at least 2 pages, got %d", len(result.Pages))
	}

	// The broken page should have status 500
	foundBroken := false
	for _, p := range result.Pages {
		if p.StatusCode == 500 {
			foundBroken = true
		}
	}
	if !foundBroken {
		t.Error("expected to find a page with status 500")
	}
}
