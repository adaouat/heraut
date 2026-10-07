//go:build !heraut_testclock

package app

import "time"

func clock() func() time.Time { return time.Now }
