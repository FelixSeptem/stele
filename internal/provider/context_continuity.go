package provider

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

type ContextContinuityConfig struct{ TTLSeconds, MaxReceiptsPerBinding, MaxReceipts, CleanupBatch int }

func DefaultContextContinuityConfig() ContextContinuityConfig {
	return ContextContinuityConfig{86400, 128, 100000, 500}
}
func (c ContextContinuityConfig) Validate() error {
	if c.TTLSeconds < 60 || c.TTLSeconds > 604800 || c.MaxReceiptsPerBinding < 1 || c.MaxReceiptsPerBinding > 4096 || c.MaxReceipts < 1 || c.MaxReceipts > 1000000 || c.MaxReceiptsPerBinding > c.MaxReceipts || c.CleanupBatch < 1 || c.CleanupBatch > 5000 {
		return fmt.Errorf("invalid provider context continuity configuration")
	}
	return nil
}

var ErrContinuityUnavailable = errors.New("continuity_reference_unavailable")
var ErrContinuityExpired = errors.New("continuity_reference_expired")
var ErrContinuityRequestChanged = errors.New("continuity_request_changed")
var ErrContinuityIncompatible = errors.New("continuity_incompatible")
var ErrContinuityCapacity = errors.New("continuity capacity exhausted")

type ContextReceipt struct {
	Reference           string
	TurnID              string
	RequestFingerprint  string
	Digest              ContextDigest
	IssuedAt, ExpiresAt time.Time
}
type ContextReceiptStore interface {
	LookupContextReceipt(context.Context, RuntimeBinding, string) (ContextReceipt, error)
	IssueContextReceipt(context.Context, RuntimeBinding, string, ContextDigest, ContextContinuityConfig, func(ContextReceipt) error) (ContextReceipt, error)
	CleanupContextReceipts(context.Context, int) (int, error)
}

// TurnContextReceiptStore atomically persists a turn-bound continuity receipt
// together with its bounded audit attribution. Implementations that do not
// provide this operation must not be used for turn-bound continuity writes.
type TurnContextReceiptStore interface {
	IssueContextReceiptForTurn(context.Context, RuntimeBinding, string, ContextDigest, string, ContextContinuityConfig, ContextTurnAttribution, func(ContextReceipt) error) (ContextReceipt, error)
}

func NewContextReference() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return "ctxc_" + hex.EncodeToString(raw[:]), nil
}
func ValidateContextReceipt(r ContextReceipt, fingerprint string) error {
	v1 := r.Digest.ContractVersion == ContextDigestVersion && r.Digest.CanonicalizationVersion == ContextCanonicalVersion
	v2 := r.Digest.ContractVersion == ContextTurnDigestVersion && r.Digest.CanonicalizationVersion == ContextTurnCanonicalVersion
	if (!v1 && !v2) || (v1 && r.TurnID != "") || (v2 && r.TurnID == "") || r.Digest.Algorithm != "sha-256" || r.Digest.SelectionPolicyVersion != ContextSelectionVersion || !ValidContextHash(r.Digest.Value) || !ValidContextHash(r.Digest.SelectionPolicyFingerprint) {
		return ErrContinuityIncompatible
	}
	if r.RequestFingerprint != fingerprint {
		return ErrContinuityRequestChanged
	}
	return nil
}
