//go:build e2e_forge

package forgeharness

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"strconv"
	"time"
)

var runIDPattern = regexp.MustCompile(`e2e-([0-9]+)-[0-9a-f]{4}`)

// NewRunID builds the marker every resource of one run carries.
func NewRunID(now time.Time, randHex string) string {
	return fmt.Sprintf("e2e-%d-%s", now.Unix(), randHex)
}

// RandomHex returns 4 random hex characters.
func RandomHex() string {
	b := make([]byte, 2)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

// RunIDTime extracts the creation time of the first run id found in s.
func RunIDTime(s string) (time.Time, bool) {
	m := runIDPattern.FindStringSubmatch(s)
	if m == nil {
		return time.Time{}, false
	}
	sec, err := strconv.ParseInt(m[1], 10, 64)
	if err != nil {
		return time.Time{}, false
	}
	return time.Unix(sec, 0), true
}
