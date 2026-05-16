package policy

import (
	"reflect"
	"testing"
)

func TestRegexFilterNamedCapture(t *testing.T) {
	f, err := NewRegexFilter(`^v(?P<v>\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?)$`, "$v")
	if err != nil {
		t.Fatal(err)
	}

	f.Apply([]string{"v1.0.0", "latest", "v1.1.0-rc.1", "1.0.0"})

	want := []string{"1.0.0", "1.1.0-rc.1"}
	if got := f.Items(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Items() = %#v, want %#v", got, want)
	}
	if got := f.GetOriginalTag("1.1.0-rc.1"); got != "v1.1.0-rc.1" {
		t.Fatalf("GetOriginalTag() = %q", got)
	}
}

func TestRegexFilterReplaceEmptyPreservesTag(t *testing.T) {
	f, err := NewRegexFilter(`^dev-.*$`, "")
	if err != nil {
		t.Fatal(err)
	}

	f.Apply([]string{"dev-1", "prod-1", "dev-2"})

	want := []string{"dev-1", "dev-2"}
	if got := f.Items(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Items() = %#v, want %#v", got, want)
	}
}

func TestRegexFilterReplaceZeroPreservesTag(t *testing.T) {
	f, err := NewRegexFilter(`^dev-.*$`, "$0")
	if err != nil {
		t.Fatal(err)
	}

	f.Apply([]string{"dev-1", "prod-1", "dev-2"})

	want := []string{"dev-1", "dev-2"}
	if got := f.Items(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Items() = %#v, want %#v", got, want)
	}
}

func TestRegexFilterEmptyList(t *testing.T) {
	f, err := NewRegexFilter(`^dev-.*$`, "")
	if err != nil {
		t.Fatal(err)
	}

	f.Apply(nil)

	if got := f.Items(); len(got) != 0 {
		t.Fatalf("Items() length = %d, want 0", len(got))
	}
}

func TestNewRegexFilterInvalidRegex(t *testing.T) {
	if _, err := NewRegexFilter(`[`, ""); err == nil {
		t.Fatal("expected invalid regex error")
	}
}
