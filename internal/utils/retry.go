package utils

import (
	"math/rand"
	"time"
)

// Retry is a utility function to retry a function a number of times with exponential backoff
// and jitter. It will return the result of the function if it succeeds, or the last error if
// it fails.
//
// If the error is a Stop, it will return the original error for later checking.
func Retry[R interface{}](attempts int, initialSleep time.Duration, f func() (R, error)) (R, error) {
	if res, err := f(); err != nil {
		if s, ok := err.(Stop); ok {
			// Return the original error for later checking
			return res, s.error
		}

		if attempts--; attempts > 0 {
			// Calculate exponential backoff
			sleep := initialSleep * (1 << uint(attempts))

			// Add some randomness to prevent creating a Thundering Herd
			jitter := time.Duration(rand.Int63n(int64(sleep)))
			sleep = sleep + jitter/2

			time.Sleep(sleep)
			return Retry[R](attempts, initialSleep, f)
		}
		return res, err
	} else {
		return res, nil
	}
}

type Stop struct {
	error
}

// NonRetryableError is a utility function to return an error that will not be retried
func NonRetryableError(err error) Stop {
	return Stop{err}
}
