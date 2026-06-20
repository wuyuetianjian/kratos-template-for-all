package service

import "testing"

func TestNormalizeClientIP(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "host label with chinese parentheses",
			in:   "192.168.1.20（desktop-01）",
			want: "192.168.1.20",
		},
		{
			name: "host label with ascii parentheses",
			in:   "192.168.1.20 (desktop-01)",
			want: "192.168.1.20",
		},
		{
			name: "host port",
			in:   "192.168.1.20:54321",
			want: "192.168.1.20",
		},
		{
			name: "forwarded chain",
			in:   "192.168.1.20, 10.0.0.1",
			want: "192.168.1.20",
		},
		{
			name: "bracketed ipv6 host port",
			in:   "[2001:db8::1]:443",
			want: "2001:db8::1",
		},
		{
			name: "bracketed ipv6",
			in:   "[2001:db8::1]",
			want: "2001:db8::1",
		},
		{
			name: "empty",
			in:   " ",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeClientIP(tt.in); got != tt.want {
				t.Fatalf("normalizeClientIP(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestClientIPFromValues(t *testing.T) {
	got := clientIPFromValues("", "10.1.2.3（host-a）", "192.168.1.20")
	if got != "10.1.2.3" {
		t.Fatalf("clientIPFromValues() = %q, want %q", got, "10.1.2.3")
	}
}
