package app

import (
	"github.com/FelixSeptem/stele/internal/provider"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProviderRoutesDisabledByDefault(t *testing.T) {
	h := NewHTTPHandler(HTTPDependencies{HTTP: HTTPRuntimeLimits{MaxRequestBodyBytes: 1 << 20}})
	r := httptest.NewRequest(http.MethodGet, "/v1/provider/capabilities", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status=%d want 404", w.Code)
	}
}

func TestProviderCapabilitiesRouteReturnsBoundedDocument(t *testing.T) {
	h := NewHTTPHandler(HTTPDependencies{ProviderEnabled: true, ProviderCapabilities: provider.Discover(provider.CapabilityInput{ProviderVersion: "provider-v1", SchemaVersion: "schema-v1"})})
	r := httptest.NewRequest(http.MethodGet, "/v1/provider/capabilities", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var got provider.CapabilityDocument
	if err := provider.DecodeStrict(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.ProviderVersion != "provider-v1" {
		t.Fatalf("doc=%+v", got)
	}
}
