package auth

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

const (
	maxPrincipalIDLength     = 512
	maxPrincipalLabelLength  = 128
	maxCredentialIDLength    = 128
	maxIdempotencyKeyLength  = 256
	credentialSecretByteSize = 32
	credentialDigestBytes    = 32
	credentialSaltBytes      = 16
	credentialPBKDF2Rounds   = 210000
)

type PrincipalRole string

const (
	PrincipalRolePublic PrincipalRole = "public"
	PrincipalRoleAdmin  PrincipalRole = "admin"
)

func (r PrincipalRole) Valid() bool {
	return r == PrincipalRolePublic || r == PrincipalRoleAdmin
}

type PrincipalStatus string

const (
	PrincipalStatusActive   PrincipalStatus = "active"
	PrincipalStatusDisabled PrincipalStatus = "disabled"
)

func (s PrincipalStatus) Valid() bool {
	return s == PrincipalStatusActive || s == PrincipalStatusDisabled
}

type CredentialStatus string

const (
	CredentialStatusActive   CredentialStatus = "active"
	CredentialStatusDisabled CredentialStatus = "disabled"
	CredentialStatusRevoked  CredentialStatus = "revoked"
)

func (s CredentialStatus) Valid() bool {
	return s == CredentialStatusActive || s == CredentialStatusDisabled || s == CredentialStatusRevoked
}

type ScopeGrantStatus string

const (
	ScopeGrantStatusActive  ScopeGrantStatus = "active"
	ScopeGrantStatusRevoked ScopeGrantStatus = "revoked"
)

func (s ScopeGrantStatus) Valid() bool {
	return s == ScopeGrantStatusActive || s == ScopeGrantStatusRevoked
}

type Principal struct {
	ID        string          `json:"id"`
	Role      PrincipalRole   `json:"role"`
	Status    PrincipalStatus `json:"status"`
	Label     string          `json:"label"`
	ExpiresAt time.Time       `json:"expires_at,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at,omitempty"`
}

func (p Principal) Validate() error {
	if err := validateIdentifier(p.ID, "principal id", maxPrincipalIDLength); err != nil {
		return err
	}
	if !p.Role.Valid() {
		return fmt.Errorf("principal role %q is invalid", p.Role)
	}
	if !p.Status.Valid() {
		return fmt.Errorf("principal status %q is invalid", p.Status)
	}
	if strings.TrimSpace(p.Label) == "" || len(p.Label) > maxPrincipalLabelLength {
		return fmt.Errorf("principal label is required and must be at most %d bytes", maxPrincipalLabelLength)
	}
	if p.CreatedAt.IsZero() {
		return fmt.Errorf("principal created at is required")
	}
	return nil
}

type Credential struct {
	ID           string           `json:"id"`
	PrincipalID  string           `json:"principal_id"`
	Status       CredentialStatus `json:"status"`
	CredentialID string           `json:"credential_id"`
	Salt         []byte           `json:"-"`
	Digest       []byte           `json:"-"`
	ExpiresAt    time.Time        `json:"expires_at,omitempty"`
	CreatedAt    time.Time        `json:"created_at"`
	DisabledAt   time.Time        `json:"disabled_at,omitempty"`
}

func (c Credential) Validate() error {
	if err := validateIdentifier(c.ID, "credential id", maxPrincipalIDLength); err != nil {
		return err
	}
	if err := validateIdentifier(c.PrincipalID, "credential principal id", maxPrincipalIDLength); err != nil {
		return err
	}
	if !c.Status.Valid() {
		return fmt.Errorf("credential status %q is invalid", c.Status)
	}
	if err := validateIdentifier(c.CredentialID, "credential lookup id", maxCredentialIDLength); err != nil {
		return err
	}
	if len(c.Salt) == 0 || len(c.Digest) == 0 {
		return fmt.Errorf("credential salt and digest are required")
	}
	if c.CreatedAt.IsZero() {
		return fmt.Errorf("credential created at is required")
	}
	return nil
}

type CredentialProjection struct {
	ID           string           `json:"id"`
	PrincipalID  string           `json:"principal_id"`
	Status       CredentialStatus `json:"status"`
	CredentialID string           `json:"credential_id"`
	ExpiresAt    time.Time        `json:"expires_at,omitempty"`
	CreatedAt    time.Time        `json:"created_at"`
	DisabledAt   time.Time        `json:"disabled_at,omitempty"`
}

func (c Credential) SafeProjection() CredentialProjection {
	return CredentialProjection{ID: c.ID, PrincipalID: c.PrincipalID, Status: c.Status, CredentialID: c.CredentialID, ExpiresAt: c.ExpiresAt, CreatedAt: c.CreatedAt, DisabledAt: c.DisabledAt}
}

type ScopeGrant struct {
	ID          string           `json:"id"`
	PrincipalID string           `json:"principal_id"`
	Scope       memory.Scope     `json:"scope"`
	Status      ScopeGrantStatus `json:"status"`
	CreatedAt   time.Time        `json:"created_at"`
	RevokedAt   time.Time        `json:"revoked_at,omitempty"`
}

type AuditRecord struct {
	ID           string       `json:"id"`
	PrincipalID  string       `json:"principal_id,omitempty"`
	CredentialID string       `json:"credential_id,omitempty"`
	Scope        memory.Scope `json:"scope,omitempty"`
	Action       string       `json:"action"`
	Result       string       `json:"result"`
	CreatedAt    time.Time    `json:"created_at"`
}

func (g ScopeGrant) Validate() error {
	if err := validateIdentifier(g.ID, "scope grant id", maxPrincipalIDLength); err != nil {
		return err
	}
	if err := validateIdentifier(g.PrincipalID, "scope grant principal id", maxPrincipalIDLength); err != nil {
		return err
	}
	if err := g.Scope.Validate(); err != nil {
		return err
	}
	if !g.Status.Valid() {
		return fmt.Errorf("scope grant status %q is invalid", g.Status)
	}
	if g.CreatedAt.IsZero() {
		return fmt.Errorf("scope grant created at is required")
	}
	return nil
}

func NewCredentialSecret(credentialID string) (string, error) {
	if err := validateIdentifier(credentialID, "credential lookup id", maxCredentialIDLength); err != nil {
		return "", err
	}
	secret := make([]byte, credentialSecretByteSize)
	if _, err := rand.Read(secret); err != nil {
		return "", fmt.Errorf("generate credential secret: %w", err)
	}
	return credentialID + "." + base64.RawURLEncoding.EncodeToString(secret), nil
}

func NewCredentialFromSecret(id, principalID, secret string, createdAt time.Time) (Credential, error) {
	credentialID, rawSecret, err := parseCredentialSecret(secret)
	if err != nil {
		return Credential{}, err
	}
	salt := make([]byte, credentialSaltBytes)
	if _, err := rand.Read(salt); err != nil {
		return Credential{}, fmt.Errorf("generate credential salt: %w", err)
	}
	digest, err := pbkdf2.Key(sha256.New, string(rawSecret), salt, credentialPBKDF2Rounds, credentialDigestBytes)
	if err != nil {
		return Credential{}, fmt.Errorf("derive credential digest: %w", err)
	}
	credential := Credential{ID: id, PrincipalID: principalID, Status: CredentialStatusActive, CredentialID: credentialID, Salt: salt, Digest: digest, CreatedAt: createdAt.UTC()}
	if err := credential.Validate(); err != nil {
		return Credential{}, err
	}
	return credential, nil
}

func (c Credential) MatchesSecret(secret string) bool {
	credentialID, rawSecret, err := parseCredentialSecret(secret)
	if err != nil || subtle.ConstantTimeCompare([]byte(credentialID), []byte(c.CredentialID)) != 1 || len(c.Salt) == 0 || len(c.Digest) == 0 {
		return false
	}
	digest, err := pbkdf2.Key(sha256.New, string(rawSecret), c.Salt, credentialPBKDF2Rounds, len(c.Digest))
	return err == nil && subtle.ConstantTimeCompare(digest, c.Digest) == 1
}

func PrincipalCredentialActive(principal Principal, credential Credential, now time.Time) bool {
	if principal.Status != PrincipalStatusActive || credential.Status != CredentialStatusActive || !credential.DisabledAt.IsZero() {
		return false
	}
	if (!principal.ExpiresAt.IsZero() && !principal.ExpiresAt.After(now)) || (!credential.ExpiresAt.IsZero() && !credential.ExpiresAt.After(now)) {
		return false
	}
	return true
}

func CredentialIDFromSecret(secret string) (string, error) {
	credentialID, _, err := parseCredentialSecret(secret)
	return credentialID, err
}

func parseCredentialSecret(secret string) (string, []byte, error) {
	parts := strings.Split(strings.TrimSpace(secret), ".")
	if len(parts) != 2 {
		return "", nil, fmt.Errorf("credential secret is invalid")
	}
	if err := validateIdentifier(parts[0], "credential lookup id", maxCredentialIDLength); err != nil {
		return "", nil, err
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || len(raw) != credentialSecretByteSize {
		return "", nil, fmt.Errorf("credential secret is invalid")
	}
	return parts[0], raw, nil
}

func ValidateIdempotencyKey(key string) error {
	if strings.TrimSpace(key) == "" || len(key) > maxIdempotencyKeyLength {
		return fmt.Errorf("idempotency key is required and must be at most %d bytes", maxIdempotencyKeyLength)
	}
	return nil
}

func validateIdentifier(value, label string, maxLength int) error {
	if strings.TrimSpace(value) == "" || len(value) > maxLength {
		return fmt.Errorf("%s is required and must be at most %d bytes", label, maxLength)
	}
	return nil
}
