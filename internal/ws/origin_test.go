package ws

import (
	"net/http/httptest"
	"testing"
)

func TestSameOrigin(t *testing.T) {
	tests := []struct {
		name   string
		host   string
		origin string
		want   bool
	}{
		{name: "same https host", host: "car.home.arpa", origin: "https://car.home.arpa", want: true},
		{name: "same host with port", host: "192.168.1.28:18081", origin: "http://192.168.1.28:18081", want: true},
		{name: "missing origin", host: "car.home.arpa", origin: "", want: true},
		{name: "foreign origin", host: "car.home.arpa", origin: "https://example.com", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "http://"+tt.host+"/ws", nil)
			if tt.origin != "" {
				r.Header.Set("Origin", tt.origin)
			}

			if got := sameOrigin(r); got != tt.want {
				t.Fatalf("sameOrigin() = %v, want %v", got, tt.want)
			}
		})
	}
}
