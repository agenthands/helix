package guardrails

import (
	"encoding/base32"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// ReceiptID is the opaque server-issued identity for a capability receipt.
// Format: "rcpt_<26 base32 chars>" (31 chars total).
// The body is UUIDv7 encoded as base32 with no padding, providing
// 128 bits of entropy with a time-ordered prefix.
type ReceiptID string

const (
	receiptIDPrefix  = "rcpt_"
	receiptIDBodyLen = 26
	receiptIDTotalLen = len(receiptIDPrefix) + receiptIDBodyLen // 31
)

// Typed errors for ParseReceiptID.
var (
	ErrInvalidReceiptIDPrefix   = errors.New("receipt ID must start with 'rcpt_'")
	ErrInvalidReceiptIDLength   = errors.New("receipt ID body must be exactly 26 characters")
	ErrInvalidReceiptIDAlphabet = errors.New("receipt ID body contains invalid base32 characters")
)

// base32NoPad is the base32 encoding without padding, using the standard
// RFC 4648 alphabet (A-Z, 2-7).
var base32NoPad = base32.StdEncoding.WithPadding(base32.NoPadding)

// NewReceiptID generates a new ReceiptID using a UUIDv7 (timestamp-prefixed,
// crypto/rand-backed) source encoded as base32 with no padding.
// Returns error only if UUID generation fails (system entropy failure).
func NewReceiptID() (ReceiptID, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("generating UUIDv7 for receipt ID: %w", err)
	}
	body := base32NoPad.EncodeToString(id[:])
	// UUIDv7 is 16 bytes; base32 without padding for 16 bytes = ceil(16*8/5) = 26 chars.
	return ReceiptID(receiptIDPrefix + strings.ToLower(body)), nil
}

// ParseReceiptID validates a receipt ID string and returns the typed ReceiptID.
// Validation rules:
//   - Must start with "rcpt_"
//   - Total length must be exactly 31 chars (prefix 5 + body 26)
//   - Body must be valid base32 alphabet (a-z, 2-7 lowercase or A-Z, 2-7 uppercase)
func ParseReceiptID(s string) (ReceiptID, error) {
	if !strings.HasPrefix(s, receiptIDPrefix) {
		return "", ErrInvalidReceiptIDPrefix
	}
	body := s[len(receiptIDPrefix):]
	if len(body) != receiptIDBodyLen {
		return "", ErrInvalidReceiptIDLength
	}
	// Validate alphabet: base32 standard alphabet (case-insensitive).
	upper := strings.ToUpper(body)
	_, err := base32NoPad.DecodeString(upper)
	if err != nil {
		return "", ErrInvalidReceiptIDAlphabet
	}
	return ReceiptID(s), nil
}
