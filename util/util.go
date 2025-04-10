// Package util provides utility functions for common tasks.
//
// Includes robust timing, randomization, and context handling utilities.
package util

import (
	"context"
	"crypto/rand"
	"fmt"
	"math"
	"math/big"
	"time"
)

// --------------------------------------------------------------------------------
// Constants

const (
	// DefaultMinWait is the minimum wait duration when invalid or zero.
	DefaultMinWait = time.Millisecond
	// DefaultMaxWait is the default maximum wait time if unspecified.
	DefaultMaxWait = 30 * time.Second
	// DefaultJitterFactor is the default fraction of wait time used for jitter.
	DefaultJitterFactor = 0.5
	// maxSafeShift prevents integer overflow in exponential backoff calculations.
	// It represents the maximum shift (2^62) before hitting int64 limits.
	maxSafeShift = 62
)

// --------------------------------------------------------------------------------
// Utility Functions

// Wait delays execution with exponential backoff and jitter.
//
// Applies an exponentially increasing delay based on the attempt number,
// capped by maxWait, with optional random jitter to prevent thundering herd
// issues. The delay is calculated as min(maxWait, base * 2^(attempt-1)) + jitter.
//
// Parameters:
//   - ctx: Context for cancellation.
//   - attempt: Retry attempt number (1-based; 0 treated as 1).
//   - base: Base wait time for the first attempt.
//   - maxWait: Maximum wait time cap (0 uses DefaultMaxWait).
//   - jitterFactor: Fraction of wait time for jitter (0 to 1; 0 disables jitter).
//
// Returns:
//   - nil if the delay completes.
//   - ctx.Err() if the context is canceled.
//
// Example:
//
//	err := Wait(context.Background(), 3, time.Second, 10*time.Second, 0.5)
//	if err != nil {
//	    log.Println("Wait canceled:", err)
//	}
func Wait(ctx context.Context, attempt uint, base, maxWait time.Duration, jitterFactor float64) error {
	// Normalize inputs.
	if base <= 0 {
		base = DefaultMinWait
	}

	if maxWait <= 0 {
		maxWait = DefaultMaxWait
	}

	if jitterFactor < 0 || jitterFactor > 1 {
		jitterFactor = DefaultJitterFactor
	}

	attempt = max(attempt, 1) // Treat 0 as 1.

	// Calculate exponential backoff: base * 2^(attempt-1).
	wait := calculateBackoff(attempt, base, maxWait)

	// Apply jitter if enabled.
	var jitter time.Duration

	if jitterFactor > 0 {
		maxJitter := time.Duration(float64(wait) * jitterFactor)

		j, err := rand.Int(rand.Reader, big.NewInt(int64(maxJitter)))
		if err != nil {
			return fmt.Errorf("failed to generate jitter: %w", err)
		}

		jitter = time.Duration(j.Int64())
	}

	// Wait with context cancellation.
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(wait + jitter):
		return nil
	}
}

// --------------------------------------------------------------------------------
// Helper Functions

// calculateBackoff computes the exponential backoff duration.
//
// It ensures the result does not exceed maxWait and handles potential overflows.
func calculateBackoff(attempt uint, base, maxWait time.Duration) time.Duration {
	shift := attempt - 1
	if shift > maxSafeShift { // Prevent overflow beyond 2^62.
		return maxWait
	}

	// Check for potential overflow before shifting.
	if maxShifted := math.MaxInt64 / base; maxShifted < 1<<shift {
		return maxWait
	}

	wait := base * (1 << shift)

	return min(wait, maxWait)
}
