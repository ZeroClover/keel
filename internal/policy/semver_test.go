package policy

import "testing"

func TestSemVerLatest(t *testing.T) {
	tests := []struct {
		name       string
		constraint string
		candidates []string
		want       string
		wantErr    bool
	}{
		{
			name:       "include prerelease",
			constraint: ">=1.0.0-0",
			candidates: []string{"1.0.0-rc.1", "1.0.0-rc.2", "0.9.0"},
			want:       "1.0.0-rc.2",
		},
		{
			name:       "exclude prerelease",
			constraint: ">=1.0.0",
			candidates: []string{"1.0.0", "1.0.0-rc.2", "1.1.0-beta.1"},
			want:       "1.0.0",
		},
		{
			name:       "caret range",
			constraint: "^1",
			candidates: []string{"1.0.0", "1.9.0", "2.0.0"},
			want:       "1.9.0",
		},
		{
			name:       "tilde range",
			constraint: "~1.2",
			candidates: []string{"1.2.0", "1.2.9", "1.3.0"},
			want:       "1.2.9",
		},
		{
			name:       "compound range",
			constraint: ">=1.0.0, <2.0",
			candidates: []string{"0.9.0", "1.5.0", "2.0.0"},
			want:       "1.5.0",
		},
		{
			name:       "empty candidates",
			constraint: ">=1.0.0",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := NewSemVer(tt.constraint)
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

func TestNewSemVerInvalidConstraint(t *testing.T) {
	if _, err := NewSemVer("not a constraint"); err == nil {
		t.Fatal("expected invalid constraint error")
	}
}
