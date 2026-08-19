package main

import (
	"os"
	"strconv"
	"syscall"
	"time"
)

// withDeepSeekLock serializes upstream DeepSeek calls across all frao-advisor
// processes. Every session shares one API key, and concurrent high-effort
// thinking calls trigger upstream throttling (2-3 minute responses that trip
// the client timeout). A single global lock keeps at most one call in flight,
// which keeps responses fast and inside the timeout.
//
// The lock is an optimization, never a correctness gate: if the lock file
// can't be created, the wait times out, or ADVISOR_SERIALIZE=0 is set, calls
// proceed unlocked rather than failing.
func withDeepSeekLock(fn func() error) error {
	if getEnv("ADVISOR_SERIALIZE", "1") == "0" {
		return fn()
	}

	path := getEnv("ADVISOR_LOCK_PATH", "/tmp/frao-advisor-deepseek.lock")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return fn()
	}
	defer f.Close()

	// Bounded wait so a call behind another long review degrades to concurrent
	// instead of blocking the tool call indefinitely.
	waitMs := 120000
	if v := getEnv("ADVISOR_LOCK_WAIT_MS", "120000"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			waitMs = n
		}
	}
	deadline := time.Now().Add(time.Duration(waitMs) * time.Millisecond)
	for {
		err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			return fn()
		}
		time.Sleep(250 * time.Millisecond)
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	return fn()
}
