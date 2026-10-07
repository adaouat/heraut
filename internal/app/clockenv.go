package app

import (
	"fmt"
	"time"
)

const testNowEnv = "HERAUT_TEST_NOW"

// clockFromEnv returns a clock pinned to the RFC 3339 instant in HERAUT_TEST_NOW, or time.Now when
// the variable is unset. Only the heraut_testclock build wires it into clock(); it is kept out of
// the tagged file so its parsing is unit-tested in the default build.
func clockFromEnv(getenv func(string) string) (func() time.Time, error) {
	raw := getenv(testNowEnv)
	if raw == "" {
		return time.Now, nil
	}
	pinned, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, fmt.Errorf("%s=%q is not an RFC 3339 instant: %w", testNowEnv, raw, err)
	}
	return func() time.Time { return pinned }, nil
}
