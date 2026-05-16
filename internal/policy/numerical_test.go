package policy

import "testing"

func TestNumericalLatest(t *testing.T) {
	tests := []struct {
		name       string
		order      string
		candidates []string
		want       string
		wantErr    bool
	}{
		{name: "default asc", candidates: []string{"10", "2", "9"}, want: "2"},
		{name: "asc negative", order: "asc", candidates: []string{"10", "-2", "9"}, want: "-2"},
		{name: "desc", order: "desc", candidates: []string{"100", "200", "50"}, want: "200"},
		{name: "empty", order: "desc", wantErr: true},
		{name: "fail fast", order: "desc", candidates: []string{"100", "abc", "200"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := NewNumerical(tt.order)
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

func TestNewNumericalInvalidOrder(t *testing.T) {
	if _, err := NewNumerical("sideways"); err == nil {
		t.Fatal("expected invalid order error")
	}
}
