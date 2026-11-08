package installer

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestReleaseAssetFallsBackToTheAPIOnServerErrors(t *testing.T) {
	downloads := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/o/tool/releases/download/v1.0.0/tool.tar.gz":
			downloads++
			w.WriteHeader(http.StatusGatewayTimeout)
		case "/repos/o/tool/releases/tags/v1.0.0":
			fmt.Fprint(w, `{"assets":[{"id":7,"name":"tool.tar.gz"}]}`)
		case "/repos/o/tool/releases/assets/7":
			if r.Header.Get("Accept") != "application/octet-stream" {
				w.WriteHeader(http.StatusNotAcceptable)
				return
			}
			fmt.Fprint(w, "bytes")
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	withBases(t, server.URL)

	blob, err := releaseAsset("o/tool", "v1.0.0", "tool.tar.gz")
	if err != nil || string(blob) != "bytes" {
		t.Fatalf("want the API bytes, got %q, %v", blob, err)
	}
	if downloads != downloadAttempts {
		t.Fatalf("want %d download attempts before the fallback, got %d", downloadAttempts, downloads)
	}
}

func TestReleaseAssetDoesNotRetryAMissingAsset(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	withBases(t, server.URL)

	if _, err := releaseAsset("o/tool", "v1.0.0", "tool.tar.gz"); err == nil {
		t.Fatal("a 404 must fail")
	}
	if calls != 1 {
		t.Fatalf("a 404 must not be retried or sent to the API, got %d calls", calls)
	}
}

func withBases(t *testing.T, url string) {
	t.Helper()
	oldGitHub, oldAPI, oldDelay := githubBase, apiBase, retryDelay
	githubBase, apiBase, retryDelay = url, url, 0
	t.Cleanup(func() { githubBase, apiBase, retryDelay = oldGitHub, oldAPI, oldDelay })
}
