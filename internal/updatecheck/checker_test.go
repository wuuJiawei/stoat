package updatecheck

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLatestReturnsOnlyNewerStableVersion(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		expectedUserAgents := []string{"stoat/v1.2.0", "stoat/v1.3.0"}
		if requests >= len(expectedUserAgents) || request.Header.Get("User-Agent") != expectedUserAgents[requests] {
			http.Error(writer, "unexpected user agent", http.StatusBadRequest)
			return
		}
		requests++
		_, _ = writer.Write([]byte("v1.3.0\n"))
	}))
	defer server.Close()

	checker := Checker{URL: server.URL, Client: server.Client()}
	latest, err := checker.Latest(context.Background(), "v1.2.0")
	if err != nil || latest != "v1.3.0" {
		t.Fatalf("unexpected update result: latest=%q err=%v", latest, err)
	}

	latest, err = checker.Latest(context.Background(), "v1.3.0")
	if err != nil || latest != "" {
		t.Fatalf("equal version should not notify: latest=%q err=%v", latest, err)
	}
	if requests != 2 {
		t.Fatalf("expected two update requests, got %d", requests)
	}
}

func TestLatestFailsClosedBeforeNetworkForDevelopmentVersion(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests++
		_, _ = writer.Write([]byte("v9.9.9"))
	}))
	defer server.Close()

	checker := Checker{URL: server.URL, Client: server.Client()}
	if _, err := checker.Latest(context.Background(), "dev"); err == nil {
		t.Fatal("expected development version rejection")
	}
	if requests != 0 {
		t.Fatalf("development build made %d request(s)", requests)
	}
}

func TestLatestRejectsInvalidOrOversizedResponse(t *testing.T) {
	for name, body := range map[string]string{
		"prerelease":  "v1.3.0-beta.1",
		"not version": "latest",
		"oversized":   strings.Repeat("1", maxResponseBytes+1),
	} {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				_, _ = writer.Write([]byte(body))
			}))
			defer server.Close()
			checker := Checker{URL: server.URL, Client: server.Client()}
			if latest, err := checker.Latest(context.Background(), "v1.2.0"); err == nil || latest != "" {
				t.Fatalf("expected closed failure: latest=%q err=%v", latest, err)
			}
		})
	}
}

func TestLatestRejectsHTTPFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	checker := Checker{URL: server.URL, Client: server.Client()}
	if _, err := checker.Latest(context.Background(), "v1.2.0"); err == nil {
		t.Fatal("expected HTTP failure")
	}
}
