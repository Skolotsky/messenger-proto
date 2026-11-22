// Package ids makes and checks the identifiers shared by all parts.
package ids

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
)

// Org identifies an organization. It is part of every key.
type Org string

var orgPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)

// ParseOrg accepts a well-formed organization ID.
func ParseOrg(s string) (Org, error) {
	if !orgPattern.MatchString(s) {
		return "", fmt.Errorf("not an organization ID: %q", s)
	}
	return Org(s), nil
}

// New returns a random ID with the given prefix, such as "msg".
func New(prefix string) string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		panic(err) // the system's random source failing is not recoverable
	}
	return prefix + "_" + hex.EncodeToString(b)
}
