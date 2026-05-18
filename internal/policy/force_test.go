package policy

import "testing"

func TestForceLatest(t *testing.T) {
	tests := []struct {
		name       string
		candidates []string
		want       string
		wantErr    bool
	}{
		{name: "empty", wantErr: true},
		{name: "single", candidates: []string{"any-tag"}, want: "any-tag"},
		{name: "multiple", candidates: []string{"first", "second"}, want: "first"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewForce().Latest(tt.candidates)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Latest() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("Latest() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestForceSortByCreated(t *testing.T) {
	plc := NewForce()
	if plc.SortByCreated() {
		t.Fatalf("default Force policy should not sort by created time")
	}

	created, err := NewForceWithOption("created")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !created.SortByCreated() {
		t.Fatalf("force:created policy should sort by created time")
	}

	if _, err := NewForceWithOption("bogus"); err == nil {
		t.Fatalf("expected error for unknown force option")
	}
}
