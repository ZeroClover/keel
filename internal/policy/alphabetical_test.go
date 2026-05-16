package policy

import "testing"

func TestAlphabeticalLatest(t *testing.T) {
	tests := []struct {
		name       string
		order      string
		candidates []string
		want       string
		wantErr    bool
	}{
		{name: "default asc", candidates: []string{"build-010", "build-002", "build-009"}, want: "build-002"},
		{name: "asc", order: "asc", candidates: []string{"dev-2", "dev-10", "dev-9"}, want: "dev-10"},
		{name: "desc", order: "desc", candidates: []string{"build-001", "build-002", "build-009", "build-010"}, want: "build-010"},
		{name: "empty", order: "desc", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := NewAlphabetical(tt.order)
			if err != nil {
				t.Fatal(err)
			}
			got, err := p.Latest(tt.candidates)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Latest() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("Latest() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNewAlphabeticalInvalidOrder(t *testing.T) {
	if _, err := NewAlphabetical("sideways"); err == nil {
		t.Fatal("expected invalid order error")
	}
}
