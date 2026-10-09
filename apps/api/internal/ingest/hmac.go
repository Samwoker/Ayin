package ingest

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"math"
	"strconv"
	"strings"
	"time"
)

// DefaultReplayWindow defines maximum allowed clock skew for webhook requests.
const DefaultReplayWindow = 5 * time.Minute

// VerifyWebhookSignature verifies HMAC-SHA256 signature and replay prevention timestamp.
func VerifyWebhookSignature(
	secret string,
	body []byte,
	signatureHeader string,
	timestampHeader string,
	maxSkew time.Duration,
) error {
	if strings.TrimSpace(signatureHeader) == "" {
		return ErrMissingSignature
	}

	if maxSkew <= 0 {
		maxSkew = DefaultReplayWindow
	}

	// 1. Replay prevention check if timestamp header is supplied
	if timestampHeader != "" {
		ts, err := parseTimestamp(timestampHeader)
		if err != nil {
			return ErrTimestampInvalid
		}

		diff := math.Abs(time.Since(ts).Seconds())
		if diff > maxSkew.Seconds() {
			return ErrTimestampExpired
		}
	}

	// 2. Extract clean hex signature
	providedSig := strings.TrimSpace(signatureHeader)
	if strings.HasPrefix(providedSig, "sha256=") {
		providedSig = strings.TrimPrefix(providedSig, "sha256=")
	} else if strings.HasPrefix(providedSig, "v1=") {
		providedSig = strings.TrimPrefix(providedSig, "v1=")
	}

	providedBytes, err := hex.DecodeString(providedSig)
	if err != nil {
		return ErrInvalidSignature
	}

	// 3. Compute expected HMAC
	secretBytes := []byte(secret)

	// Mode A: Raw body HMAC
	macA := hmac.New(sha256.New, secretBytes)
	macA.Write(body)
	expectedA := macA.Sum(nil)

	if subtle.ConstantTimeCompare(providedBytes, expectedA) == 1 {
		return nil
	}

	// Mode B: Timestamp-prefixed body HMAC (e.g. Stripe/Slack standard: timestamp + "." + body)
	if timestampHeader != "" {
		macB := hmac.New(sha256.New, secretBytes)
		macB.Write([]byte(timestampHeader))
		macB.Write([]byte("."))
		macB.Write(body)
		expectedB := macB.Sum(nil)

		if subtle.ConstantTimeCompare(providedBytes, expectedB) == 1 {
			return nil
		}
	}

	return ErrInvalidSignature
}

// ComputeWebhookSignature generates an HMAC-SHA256 signature for testing and outbound hooks.
func ComputeWebhookSignature(secret string, body []byte, timestampHeader string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	if timestampHeader != "" {
		mac.Write([]byte(timestampHeader))
		mac.Write([]byte("."))
	}
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func parseTimestamp(tsStr string) (time.Time, error) {
	tsStr = strings.TrimSpace(tsStr)

	// Try numeric epoch seconds or milliseconds
	if n, err := strconv.ParseInt(tsStr, 10, 64); err == nil {
		if n > 1e11 { // milliseconds
			return time.UnixMilli(n), nil
		}
		return time.Unix(n, 0), nil
	}

	// Try RFC3339
	if t, err := time.Parse(time.RFC3339, tsStr); err == nil {
		return t, nil
	}

	return time.Time{}, ErrTimestampInvalid
}
