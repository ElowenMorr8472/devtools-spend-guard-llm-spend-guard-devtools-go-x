package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestChatAcceptsOpenAIResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Write([]byte(`{"id":"chat-1","choices":[{"message":{"role":"assistant","content":"hello"}}]}`))
	}))
	defer server.Close()
	c := &Client{BaseURL: server.URL, HTTP: server.Client()}
	got, err := c.Chat("hi")
	if err != nil || !strings.Contains(string(got), `"chat-1"`) {
		t.Fatalf("chat response = %s, error = %v", got, err)
	}
}

func TestUsageTimeseriesRejectsErrorEnvelope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"ok":false,"error":{"code":"UNAVAILABLE"}}`))
	}))
	defer server.Close()
	c := &Client{BaseURL: server.URL, HTTP: server.Client()}
	if _, err := c.UsageTimeseries(); err == nil {
		t.Fatal("expected usage error")
	}
}

func TestUnderCap(t *testing.T) {
	tests := []struct {
		name                 string
		spent, estimate, cap float64
		want                 bool
	}{
		{"room", 4, 1, 5, true}, {"exact cap", 4, 1, 5, true}, {"over", 4.01, 1, 5, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := underCap(tt.spent, tt.estimate, tt.cap); got != tt.want {
				t.Fatalf("got %v want %v", got, tt.want)
			}
		})
	}
}
