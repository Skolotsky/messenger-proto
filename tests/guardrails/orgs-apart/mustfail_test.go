//go:build t057faults

package orgsapart

import (
	"testing"

	"contexia/tests/e2e/stack"
)

// The door check must notice content that is there: asked to find an organization's
// own secret as if it were foreign, it has to fail. The planted query's failing case
// runs in the ordinary test, with the row rules off.
func TestFailsWhenADoorHoldsTheWordItLooksFor(t *testing.T) {
	_, acme, _ := setup(t)
	stack.MustFail(t, "holds something of the other organization", func(tb testing.TB) {
		nothingOf(tb, acme, acme.secret)
	})
}
