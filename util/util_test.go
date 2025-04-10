// Package util_test contains tests for the util package.
//
// Verifies timing, backoff, jitter, and context handling in Wait function.
package util_test

import (
	"context"
	"testing"
	"time"

	"github.com/qntx/gonet/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --------------------------------------------------------------------------------
// Constants

const (
	// tolerance accounts for system timing variations and jitter.
	tolerance = 50 * time.Millisecond
)

// --------------------------------------------------------------------------------
// Tests

// TestWait verifies Wait's exponential backoff, jitter, and cancellation behavior.
func TestWait(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		ctx     context.Context
		attempt uint
		base    time.Duration
		maxWait time.Duration
		jitter  float64
		wantMin time.Duration
		wantMax time.Duration
		wantErr error
	}{
		{
			name:    "FirstAttempt",
			ctx:     t.Context(),
			attempt: 1,
			base:    100 * time.Millisecond,
			maxWait: 2 * time.Second,
			jitter:  0.5,
			wantMin: 100 * time.Millisecond,
			wantMax: 150*time.Millisecond + tolerance,
			wantErr: nil,
		},
		{
			name:    "ExponentialBackoff",
			ctx:     t.Context(),
			attempt: 3,
			base:    100 * time.Millisecond,
			maxWait: 1 * time.Second,
			jitter:  0.5,
			wantMin: 400 * time.Millisecond,
			wantMax: 600*time.Millisecond + tolerance,
			wantErr: nil,
		},
		{
			name:    "MaxCapped",
			ctx:     t.Context(),
			attempt: 10,
			base:    10 * time.Millisecond,
			maxWait: 1 * time.Second,
			jitter:  0.5,
			wantMin: 1 * time.Second,
			wantMax: 1500*time.Millisecond + tolerance,
			wantErr: nil,
		},
		{
			name:    "ZeroAttempt",
			ctx:     t.Context(),
			attempt: 0,
			base:    100 * time.Millisecond,
			maxWait: 2 * time.Second,
			jitter:  0.5,
			wantMin: 100 * time.Millisecond,
			wantMax: 150*time.Millisecond + tolerance,
			wantErr: nil,
		},
		{
			name:    "ZeroBase",
			ctx:     t.Context(),
			attempt: 1,
			base:    0,
			maxWait: 0,
			jitter:  0.5,
			wantMin: util.DefaultMinWait,
			wantMax: util.DefaultMinWait + tolerance,
			wantErr: nil,
		},
		{
			name:    "NoJitter",
			ctx:     t.Context(),
			attempt: 2,
			base:    100 * time.Millisecond,
			maxWait: 1 * time.Second,
			jitter:  0,
			wantMin: 200 * time.Millisecond,
			wantMax: 200*time.Millisecond + tolerance,
			wantErr: nil,
		},
		{
			name:    "ContextCancel",
			ctx:     canceledContext(),
			attempt: 1,
			base:    100 * time.Millisecond,
			maxWait: 2 * time.Second,
			jitter:  0.5,
			wantMin: 0,
			wantMax: 10 * time.Millisecond,
			wantErr: context.Canceled,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			start := time.Now()
			err := util.Wait(tt.ctx, tt.attempt, tt.base, tt.maxWait, tt.jitter)
			duration := time.Since(start)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.LessOrEqual(t, duration, tt.wantMax, "Canceled wait should be quick")

				return
			}

			require.NoError(t, err)
			assert.GreaterOrEqual(t, duration, tt.wantMin, "Wait duration too short")
			assert.LessOrEqual(t, duration, tt.wantMax, "Wait duration too long")
		})
	}
}

// --------------------------------------------------------------------------------
// Helpers

// canceledContext creates a pre-canceled context for testing.
func canceledContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	return ctx
}
