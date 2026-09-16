package provider

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/FelixSeptem/stele/internal/auth"
	"github.com/FelixSeptem/stele/internal/memory"
)

const (
	HeaderRuntimeBinding      = "X-Stele-Runtime-Binding"
	HeaderRuntimeSession      = "X-Stele-Runtime-Session"
	HeaderRuntimeAgent        = "X-Stele-Runtime-Agent"
	HeaderRuntimeConversation = "X-Stele-Runtime-Conversation"
	maxRuntimeIdentityLength  = 256
	maxRuntimeBindingLength   = 256
)

// RuntimeInitialization is the client-supplied identity context for creating
// a provider runtime. Scope is checked against the authenticated principal's
// exact grant and never widened by the provider layer.
type RuntimeInitialization struct {
	Scope              memory.Scope `json:"scope"`
	AgentID            string       `json:"agent_id"`
	SessionID          string       `json:"session_id"`
	ConversationID     string       `json:"conversation_id,omitempty"`
	ProviderInstanceID string       `json:"provider_instance_id,omitempty"`
}

func (i RuntimeInitialization) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	for value, label := range map[string]string{i.AgentID: "agent id", i.SessionID: "session id"} {
		if strings.TrimSpace(value) == "" || len(value) > maxRuntimeIdentityLength {
			return fmt.Errorf("%s is required and must be at most %d bytes", label, maxRuntimeIdentityLength)
		}
	}
	if i.ProviderInstanceID != "" && len(i.ProviderInstanceID) > maxRuntimeIdentityLength {
		return fmt.Errorf("provider instance id must be at most %d bytes", maxRuntimeIdentityLength)
	}
	return nil
}

// RuntimeBinding is the server-owned opaque binding carried on every provider
// operation. Callers must use Scope and identities returned by the server.
type RuntimeBinding struct {
	BindingID          string       `json:"binding_id"`
	PrincipalID        string       `json:"principal_id"`
	Scope              memory.Scope `json:"scope"`
	AgentID            string       `json:"agent_id"`
	SessionID          string       `json:"session_id"`
	ConversationID     string       `json:"conversation_id"`
	ProviderInstanceID string       `json:"provider_instance_id"`
	CreatedAt          time.Time    `json:"created_at"`
	ExpiresAt          time.Time    `json:"expires_at"`
	RevokedAt          time.Time    `json:"revoked_at,omitempty"`
}

func (b RuntimeBinding) Validate(now time.Time) error {
	if strings.TrimSpace(b.BindingID) == "" || len(b.BindingID) > maxRuntimeBindingLength {
		return fmt.Errorf("runtime binding is invalid")
	}
	if strings.TrimSpace(b.PrincipalID) == "" {
		return fmt.Errorf("runtime principal is invalid")
	}
	if err := b.Scope.Validate(); err != nil {
		return err
	}
	if !runtimeIdentityValid(b.AgentID, true) || !runtimeIdentityValid(b.SessionID, true) || !runtimeIdentityValid(b.ConversationID, false) || !runtimeIdentityValid(b.ProviderInstanceID, true) {
		return fmt.Errorf("runtime identity is invalid")
	}
	if b.CreatedAt.IsZero() || b.ExpiresAt.IsZero() {
		return fmt.Errorf("runtime binding timestamps are required")
	}
	if !b.RevokedAt.IsZero() || !b.ExpiresAt.After(now) {
		return fmt.Errorf("runtime binding is expired or revoked")
	}
	return nil
}

func runtimeIdentityValid(value string, required bool) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return !required
	}
	return len(value) <= maxRuntimeIdentityLength && boundedToken(value, maxRuntimeIdentityLength)
}

// RuntimeBindingStore allows deployments to persist bindings in their
// existing durable store. Implementations must enforce opaque ID lookup.
type RuntimeBindingStore interface {
	Create(context.Context, RuntimeBinding) error
	Lookup(context.Context, string) (RuntimeBinding, error)
}

// MemoryBindingStore is a bounded in-process store intended for tests and
// single-process development. Production deployments should provide a
// PostgreSQL-backed RuntimeBindingStore.
type MemoryBindingStore struct {
	mu       sync.RWMutex
	bindings map[string]RuntimeBinding
}

func NewMemoryBindingStore() *MemoryBindingStore {
	return &MemoryBindingStore{bindings: make(map[string]RuntimeBinding)}
}
func (s *MemoryBindingStore) Create(_ context.Context, b RuntimeBinding) error {
	if s == nil {
		return fmt.Errorf("runtime binding store is not configured")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.bindings[b.BindingID]; ok {
		return fmt.Errorf("runtime binding already exists")
	}
	s.bindings[b.BindingID] = b
	return nil
}
func (s *MemoryBindingStore) Lookup(_ context.Context, id string) (RuntimeBinding, error) {
	if s == nil {
		return RuntimeBinding{}, fmt.Errorf("runtime binding store is not configured")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	b, ok := s.bindings[strings.TrimSpace(id)]
	if !ok {
		return RuntimeBinding{}, fmt.Errorf("runtime binding not found")
	}
	return b, nil
}
func (s *MemoryBindingStore) Revoke(id string, at time.Time) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if b, ok := s.bindings[id]; ok {
		b.RevokedAt = at.UTC()
		s.bindings[id] = b
	}
}
func (s *MemoryBindingStore) Count() int {
	if s == nil {
		return 0
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.bindings)
}

type RuntimeInitializerOptions struct {
	Authorizer auth.PrincipalAuthorizer
	Bindings   RuntimeBindingStore
	BindingTTL time.Duration
	Now        func() time.Time
}
type RuntimeInitializer struct {
	authorizer auth.PrincipalAuthorizer
	bindings   RuntimeBindingStore
	ttl        time.Duration
	now        func() time.Time
}

func NewRuntimeInitializer(options RuntimeInitializerOptions) *RuntimeInitializer {
	now := options.Now
	if now == nil {
		now = time.Now
	}
	ttl := options.BindingTTL
	if ttl <= 0 {
		ttl = time.Hour
	}
	return &RuntimeInitializer{authorizer: options.Authorizer, bindings: options.Bindings, ttl: ttl, now: now}
}

func (i *RuntimeInitializer) InitializeAuthenticated(ctx context.Context, secret string, input RuntimeInitialization) (RuntimeBinding, error) {
	if i == nil || i.authorizer == nil {
		return RuntimeBinding{}, fmt.Errorf("principal authorization is not configured")
	}
	principal, _, err := i.authorizer.Authenticate(ctx, strings.TrimSpace(secret))
	if err != nil {
		return RuntimeBinding{}, fmt.Errorf("unauthorized")
	}
	return i.Initialize(ctx, principal, input)
}

func (i *RuntimeInitializer) Initialize(ctx context.Context, principal auth.Principal, input RuntimeInitialization) (RuntimeBinding, error) {
	if i == nil || i.bindings == nil || i.authorizer == nil {
		return RuntimeBinding{}, fmt.Errorf("runtime provider is not configured")
	}
	if err := input.Validate(); err != nil {
		return RuntimeBinding{}, err
	}
	if principal.Status != auth.PrincipalStatusActive || strings.TrimSpace(principal.ID) == "" {
		return RuntimeBinding{}, fmt.Errorf("unauthorized")
	}
	scope := input.Scope.Normalized()
	granted, err := i.authorizer.AuthorizeScope(ctx, principal.ID, scope)
	if err != nil || !granted {
		return RuntimeBinding{}, fmt.Errorf("forbidden")
	}
	now := i.now().UTC()
	instance, err := opaqueID("pi_")
	if err != nil {
		return RuntimeBinding{}, fmt.Errorf("generate runtime identity: %w", err)
	}
	bindingID, err := opaqueID("rb_")
	if err != nil {
		return RuntimeBinding{}, fmt.Errorf("generate runtime binding: %w", err)
	}
	binding := RuntimeBinding{BindingID: bindingID, PrincipalID: principal.ID, Scope: scope, AgentID: strings.TrimSpace(input.AgentID), SessionID: strings.TrimSpace(input.SessionID), ConversationID: strings.TrimSpace(input.ConversationID), ProviderInstanceID: instance, CreatedAt: now, ExpiresAt: now.Add(i.ttl)}
	if err := i.bindings.Create(ctx, binding); err != nil {
		return RuntimeBinding{}, fmt.Errorf("persist runtime binding: %w", err)
	}
	return binding, nil
}

func opaqueID(prefix string) (string, error) {
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("secure random source unavailable: %w", err)
	}
	return prefix + base64.RawURLEncoding.EncodeToString(raw), nil
}

type runtimeBindingContextKey struct{}

func RuntimeBindingFromContext(ctx context.Context) (RuntimeBinding, bool) {
	b, ok := ctx.Value(runtimeBindingContextKey{}).(RuntimeBinding)
	return b, ok
}

// ValidateRuntimeOperation resolves and validates a binding for non-HTTP
// provider handlers. It applies the same exact-scope and grant checks as the
// HTTP middleware and returns the canonical server-owned binding.
func ValidateRuntimeOperation(ctx context.Context, store RuntimeBindingStore, authorizer auth.PrincipalAuthorizer, bindingID, principalID, sessionID string, requestedScope *memory.Scope) (RuntimeBinding, error) {
	if store == nil || authorizer == nil {
		return RuntimeBinding{}, fmt.Errorf("forbidden")
	}
	id := strings.TrimSpace(bindingID)
	if id == "" || len(id) > maxRuntimeBindingLength {
		return RuntimeBinding{}, fmt.Errorf("forbidden")
	}
	binding, err := store.Lookup(ctx, id)
	if err != nil || binding.Validate(time.Now().UTC()) != nil {
		return RuntimeBinding{}, fmt.Errorf("forbidden")
	}
	if strings.TrimSpace(principalID) != "" && strings.TrimSpace(principalID) != binding.PrincipalID {
		return RuntimeBinding{}, fmt.Errorf("forbidden")
	}
	if strings.TrimSpace(sessionID) != "" && strings.TrimSpace(sessionID) != binding.SessionID {
		return RuntimeBinding{}, fmt.Errorf("forbidden")
	}
	if requestedScope != nil && requestedScope.Normalized() != binding.Scope.Normalized() {
		return RuntimeBinding{}, fmt.Errorf("forbidden")
	}
	granted, err := authorizer.AuthorizeScope(ctx, binding.PrincipalID, binding.Scope)
	if err != nil || !granted {
		return RuntimeBinding{}, fmt.Errorf("forbidden")
	}
	return binding, nil
}

// RuntimeBindingMiddleware validates server-owned binding, principal, session,
// and exact scope before invoking any provider operation. All failures use a
// generic forbidden response to avoid existence disclosure.
func RuntimeBindingMiddleware(store RuntimeBindingStore, authorizer auth.PrincipalAuthorizer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if store == nil || authorizer == nil {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			id := strings.TrimSpace(r.Header.Get(HeaderRuntimeBinding))
			if id == "" || len(id) > maxRuntimeBindingLength {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			binding, err := store.Lookup(r.Context(), id)
			if err != nil || binding.Validate(time.Now().UTC()) != nil {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			principal, ok := auth.PrincipalFromContext(r.Context())
			if !ok {
				var credential auth.Credential
				principal, credential, err = authorizer.Authenticate(r.Context(), strings.TrimSpace(r.Header.Get(auth.HeaderAPIKey)))
				if err != nil || !auth.PrincipalCredentialActive(principal, credential, time.Now().UTC()) {
					http.Error(w, "forbidden", http.StatusForbidden)
					return
				}
			}
			if principal.ID != binding.PrincipalID || principal.Status != auth.PrincipalStatusActive {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			if session := strings.TrimSpace(r.Header.Get(HeaderRuntimeSession)); session == "" || session != binding.SessionID {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			if agent := strings.TrimSpace(r.Header.Get(HeaderRuntimeAgent)); agent != "" && agent != binding.AgentID {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			if conversation := strings.TrimSpace(r.Header.Get(HeaderRuntimeConversation)); conversation != "" && conversation != binding.ConversationID {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			requested := memory.Scope{Tenant: r.Header.Get(auth.HeaderTenant), Project: r.Header.Get(auth.HeaderProject), Namespace: r.Header.Get(auth.HeaderNamespace)}
			if requested.Tenant != "" || requested.Project != "" || requested.Namespace != "" {
				if requested.Normalized() != binding.Scope.Normalized() {
					http.Error(w, "forbidden", http.StatusForbidden)
					return
				}
			}
			granted, err := authorizer.AuthorizeScope(r.Context(), binding.PrincipalID, binding.Scope)
			if err != nil || !granted {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			ctx := context.WithValue(r.Context(), runtimeBindingContextKey{}, binding)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
