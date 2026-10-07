//go:build heraut_testclock

package app

import (
	"os"
	"time"
)

func clock() func() time.Time {
	now, err := clockFromEnv(os.Getenv)
	if err != nil {
		panic(err)
	}
	return now
}
