package conn

import (
	"reflect"
	"testing"

	googleoauth "golang.org/x/oauth2/google"
	"google.golang.org/api/option"
)

// clientOptions is the single place credentials reach every Spanner client;
// these cases are the regression tests for the review ARCH-2 live bug where
// provider-resolved credentials were stored and silently ignored (ADC always).
func TestClientOptions(t *testing.T) {
	creds := &googleoauth.Credentials{ProjectID: "test-project"}

	t.Run("credentials set produces WithCredentials", func(t *testing.T) {
		opts := clientOptions(creds, "")
		if len(opts) != 1 {
			t.Fatalf("got %d options, want exactly 1 (WithCredentials)", len(opts))
		}
		if want := option.WithCredentials(creds); !reflect.DeepEqual(opts[0], want) {
			t.Errorf("option = %#v, want WithCredentials(creds)", opts[0])
		}
	})

	t.Run("nil credentials means ADC (no options)", func(t *testing.T) {
		if opts := clientOptions(nil, ""); len(opts) != 0 {
			t.Errorf("got %d options, want 0 so clients fall back to ADC", len(opts))
		}
	})

	t.Run("emulator host suppresses credentials", func(t *testing.T) {
		// Passing WithCredentials alongside the emulator's insecure channel
		// causes dial conflicts; the emulator ignores auth entirely.
		if opts := clientOptions(creds, "localhost:9010"); len(opts) != 0 {
			t.Errorf("got %d options, want 0 when SPANNER_EMULATOR_HOST is set", len(opts))
		}
	})
}
