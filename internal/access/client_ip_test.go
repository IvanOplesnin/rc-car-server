package access

import "testing"

func TestClientIPResolver(t *testing.T) {
	resolver, err := NewClientIPResolver([]string{"192.168.1.25", "10.10.0.0/24"})
	if err != nil {
		t.Fatalf("create resolver: %v", err)
	}

	tests := []struct {
		name         string
		remoteAddr   string
		forwardedFor string
		want         string
	}{
		{
			name:         "trusted caddy",
			remoteAddr:   "192.168.1.25:54321",
			forwardedFor: "192.168.1.18",
			want:         "192.168.1.18",
		},
		{
			name:         "right-most value prevents spoofing",
			remoteAddr:   "192.168.1.25:54321",
			forwardedFor: "203.0.113.50, 10.10.0.2",
			want:         "10.10.0.2",
		},
		{
			name:         "untrusted peer cannot spoof",
			remoteAddr:   "192.168.1.99:54321",
			forwardedFor: "192.168.1.18",
			want:         "192.168.1.99:54321",
		},
		{
			name:         "trusted peer without header",
			remoteAddr:   "192.168.1.25:54321",
			forwardedFor: "",
			want:         "192.168.1.25:54321",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolver.Resolve(tt.remoteAddr, tt.forwardedFor); got != tt.want {
				t.Fatalf("Resolve() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestClientIPResolverRejectsInvalidProxy(t *testing.T) {
	if _, err := NewClientIPResolver([]string{"not-an-address"}); err == nil {
		t.Fatal("expected invalid trusted proxy to be rejected")
	}
}
