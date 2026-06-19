package server

import "testing"

func TestAuthOperationAllowlistContains(t *testing.T) {
	allowlist := newAuthAllowlist(
		[]string{"/temperate.v1.TemperateService/Health"},
		[]string{"/temperate.v1.PublicService/"},
	)

	tests := []struct {
		name      string
		operation string
		want      bool
	}{
		{
			name:      "exact operation",
			operation: "/temperate.v1.TemperateService/Health",
			want:      true,
		},
		{
			name:      "prefix operation",
			operation: "/temperate.v1.PublicService/Ping",
			want:      true,
		},
		{
			name:      "protected operation",
			operation: "/temperate.v1.TemperateService/CreateIncident",
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := allowlist.Contains(tt.operation); got != tt.want {
				t.Fatalf("Contains(%q) = %v, want %v", tt.operation, got, tt.want)
			}
		})
	}
}
