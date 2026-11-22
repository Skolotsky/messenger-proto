package ids

import (
	"strings"
	"testing"
)

func TestParseOrg(t *testing.T) {
	for _, ok := range []string{"acme", "org-2", "0"} {
		if _, err := ParseOrg(ok); err != nil {
			t.Errorf("%q refused: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "-a", "A", "a b", "a'; drop", strings.Repeat("a", 64)} {
		if _, err := ParseOrg(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestNewIsPrefixedAndDistinct(t *testing.T) {
	a, b := New("msg"), New("msg")
	if a == b || !strings.HasPrefix(a, "msg_") {
		t.Fatalf("%q %q", a, b)
	}
}
