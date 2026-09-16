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
	if strings.TrimSpace(i.AgentID) == "" || len(i.AgentID) > maxRuntimeIdentityLength {
		return fmt.Errorf("agent id is required and must be at most %d bytes", maxRuntimeIdentityLength)
	}
	if strings.TrimSpace(i.SessionID) == "" || len(i.SessionID) > maxRuntimeIdentityLength {
		return fmt.Errorf("session id is required and must be at most %d bytes", maxRuntimeIdentityLength)
	}
	if len(i.ConversationID) > maxRuntimeIdentityLength {
		return fmt.Errorf("conversation id must be at most %d bytes", maxRuntimeIdentityLength)
	}
	if len(i.ProviderInstanceID) > maxRuntimeIdentityLength {
		return fmt.Errorf("provider instance id must be at most %d bytes", maxRuntimeIdentityLength)
	}
	return nil
}

type RuntimeBinding struct {
	BindingID          string       `json:"binding_id"`
	PrincipalID        string       `json:"-"`
	Scope              memory.Scope `json:"scope"`
	AgentID            string       `json:"agent_id"`
	SessionID          string       `json:"session_id"`
	ConversationID     string       `json:"conversation_id,omitempty"`
	ProviderInstanceID string       `json:"provider_instance_id"`
	CreatedAt          time.Time    `json:"created_at"`
	ExpiresAt          time.Time    `json:"expires_at"`
	RevokedAt          time.Time    `json:"-"`
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
	if strings.TrimSpace(b.AgentID) == "" || strings.TrimSpace(b.SessionID) == "" {
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

type RuntimeBindingStore interface {
	Create(context.Context, RuntimeBinding) error
	Lookup(context.Context, string) (RuntimeBinding, error)
}
type MemoryBindingStore struct {
	mu       sync.RWMutex
	bindings map[string]RuntimeBinding
}

func NewMemoryBindingStore() *MemoryBindingStore {
	return &MemoryBindingStore{bindings: map[string]RuntimeBinding{}}
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

func NewRuntimeInitializer(o RuntimeInitializerOptions) *RuntimeInitializer {
	n := o.Now
	if n == nil {
		n = time.Now
	}
	ttl := o.BindingTTL
	if ttl <= 0 {
		ttl = time.Hour
	}
	return &RuntimeInitializer{o.Authorizer, o.Bindings, ttl, n}
}
func (i *RuntimeInitializer) InitializeAuthenticated(ctx context.Context, secret string, input RuntimeInitialization) (RuntimeBinding, error) {
	if i == nil || i.authorizer == nil {
		return RuntimeBinding{}, fmt.Errorf("principal authorization is not configured")
	}
	p, _, err := i.authorizer.Authenticate(ctx, strings.TrimSpace(secret))
	if err != nil {
		return RuntimeBinding{}, fmt.Errorf("unauthorized")
	}
	return i.Initialize(ctx, p, input)
}
func (i *RuntimeInitializer) Initialize(ctx context.Context, p auth.Principal, input RuntimeInitialization) (RuntimeBinding, error) {
	if i == nil || i.bindings == nil || i.authorizer == nil {
		return RuntimeBinding{}, fmt.Errorf("runtime provider is not configured")
	}
	if err := input.Validate(); err != nil {
		return RuntimeBinding{}, err
	}
	if p.Status != auth.PrincipalStatusActive || strings.TrimSpace(p.ID) == "" {
		return RuntimeBinding{}, fmt.Errorf("unauthorized")
	}
	scope := input.Scope.Normalized()
	ok, err := i.authorizer.AuthorizeScope(ctx, p.ID, scope)
	if err != nil || !ok {
		return RuntimeBinding{}, fmt.Errorf("forbidden")
	}
	now := i.now().UTC()
	inst := strings.TrimSpace(input.ProviderInstanceID)
	if inst == "" {
		inst = opaqueID("pi_")
	}
	b := RuntimeBinding{BindingID: opaqueID("rb_"), PrincipalID: p.ID, Scope: scope, AgentID: strings.TrimSpace(input.AgentID), SessionID: strings.TrimSpace(input.SessionID), ConversationID: strings.TrimSpace(input.ConversationID), ProviderInstanceID: inst, CreatedAt: now, ExpiresAt: now.Add(i.ttl)}
	if err := i.bindings.Create(ctx, b); err != nil {
		return RuntimeBinding{}, fmt.Errorf("persist runtime binding: %w", err)
	}
	return b, nil
}
func opaqueID(prefix string) string {
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return prefix + fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return prefix + base64.RawURLEncoding.EncodeToString(raw)
}

type runtimeBindingContextKey struct{}

func RuntimeBindingFromContext(ctx context.Context) (RuntimeBinding, bool) {
	b, ok := ctx.Value(runtimeBindingContextKey{}).(RuntimeBinding)
	return b, ok
}
func ValidateRuntimeOperation(ctx context.Context, store RuntimeBindingStore, authorizer auth.PrincipalAuthorizer, bindingID, principalID, sessionID string, requestedScope *memory.Scope) (RuntimeBinding, error) {
	if store == nil || authorizer == nil {
		return RuntimeBinding{}, fmt.Errorf("forbidden")
	}
	id := strings.TrimSpace(bindingID)
	if id == "" || len(id) > maxRuntimeBindingLength {
		return RuntimeBinding{}, fmt.Errorf("forbidden")
	}
	b, err := store.Lookup(ctx, id)
	if err != nil || b.Validate(time.Now().UTC()) != nil {
		return RuntimeBinding{}, fmt.Errorf("forbidden")
	}
	if strings.TrimSpace(principalID) != "" && strings.TrimSpace(principalID) != b.PrincipalID {
		return RuntimeBinding{}, fmt.Errorf("forbidden")
	}
	if strings.TrimSpace(sessionID) != "" && strings.TrimSpace(sessionID) != b.SessionID {
		return RuntimeBinding{}, fmt.Errorf("forbidden")
	}
	if requestedScope != nil && requestedScope.Normalized() != b.Scope.Normalized() {
		return RuntimeBinding{}, fmt.Errorf("forbidden")
	}
	ok, err := authorizer.AuthorizeScope(ctx, b.PrincipalID, b.Scope)
	if err != nil || !ok {
		return RuntimeBinding{}, fmt.Errorf("forbidden")
	}
	return b, nil
}
func RuntimeBindingMiddleware(store RuntimeBindingStore, authorizer auth.PrincipalAuthorizer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sessionID := strings.TrimSpace(r.Header.Get(HeaderRuntimeSession))
			if sessionID == "" {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			b, err := ValidateRuntimeOperation(r.Context(), store, authorizer, r.Header.Get(HeaderRuntimeBinding), "", sessionID, nil)
			if err != nil {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			p, ok := auth.PrincipalFromContext(r.Context())
			if !ok {
				var c auth.Credential
				p, c, err = authorizer.Authenticate(r.Context(), strings.TrimSpace(r.Header.Get(auth.HeaderAPIKey)))
				if err != nil || !auth.PrincipalCredentialActive(p, c, time.Now().UTC()) {
					http.Error(w, "forbidden", http.StatusForbidden)
					return
				}
			}
			if p.ID != b.PrincipalID || p.Status != auth.PrincipalStatusActive {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			if a := strings.TrimSpace(r.Header.Get(HeaderRuntimeAgent)); a != "" && a != b.AgentID {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			if c := strings.TrimSpace(r.Header.Get(HeaderRuntimeConversation)); c != "" && c != b.ConversationID {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			req := memory.Scope{Tenant: r.Header.Get(auth.HeaderTenant), Project: r.Header.Get(auth.HeaderProject), Namespace: r.Header.Get(auth.HeaderNamespace)}
			if req.Tenant != "" || req.Project != "" || req.Namespace != "" {
				if req.Normalized() != b.Scope.Normalized() {
					http.Error(w, "forbidden", http.StatusForbidden)
					return
				}
			}
			ctx := context.WithValue(r.Context(), runtimeBindingContextKey{}, b)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
