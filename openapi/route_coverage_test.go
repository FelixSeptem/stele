package openapi

import (
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

// Keep route registration and the published contract in lockstep. The
// registration syntax is deliberately constrained to net/http ServeMux
// patterns, so this check catches a newly added public route that was not
// added to the authoritative OpenAPI document.
func TestRegisteredRoutesArePublishedInOpenAPI(t *testing.T) {
	source, err := os.ReadFile("../internal/app/http.go")
	if err != nil {
		t.Fatalf("read HTTP route source: %v", err)
	}
	re := regexp.MustCompile(`mux\.Handle(?:Func)?\("([A-Z]+) ([^"]+)"`)
	matches := re.FindAllStringSubmatch(string(source), -1)
	if len(matches) == 0 {
		t.Fatal("no ServeMux routes found")
	}
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromData([]byte(SpecYAML()))
	if err != nil {
		t.Fatalf("parse OpenAPI: %v", err)
	}
	missing := make([]string, 0)
	for _, match := range matches {
		method, path := strings.ToLower(match[1]), match[2]
		op := findPublishedOperation(doc, path, method)
		if op == nil {
			missing = append(missing, method+" "+path)
			continue
		}
		if strings.HasPrefix(path, "/v1/") {
			assertProtectedContract(t, path, op)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Fatalf("registered routes missing from OpenAPI: %s", strings.Join(missing, ", "))
	}
}

func routeOperationPublished(doc *openapi3.T, route, method string) bool {
	return findPublishedOperation(doc, route, method) != nil
}

func findPublishedOperation(doc *openapi3.T, route, method string) *openapi3.Operation {
	if item := doc.Paths.Value(route); item != nil && operationForMethod(item, method) != nil {
		return operationForMethod(item, method)
	}
	pattern := regexp.QuoteMeta(route)
	pattern = regexp.MustCompile(`\\\{[^}]+\\\}`).ReplaceAllString(pattern, `[^/]+`)
	re := regexp.MustCompile("^" + pattern + "$")
	for path, item := range doc.Paths.Map() {
		if re.MatchString(path) && operationForMethod(item, method) != nil {
			return operationForMethod(item, method)
		}
	}
	return nil
}

func assertProtectedContract(t *testing.T, path string, op *openapi3.Operation) {
	t.Helper()
	names := map[string]bool{}
	for _, parameter := range op.Parameters {
		if parameter == nil {
			continue
		}
		if parameter.Value != nil {
			names[parameter.Value.Name] = true
		}
		switch parameter.Ref {
		case "#/components/parameters/PublicAPIKey":
			names["X-Stele-API-Key"] = true
		case "#/components/parameters/AdminAPIKey":
			names["X-Stele-Admin-Key"] = true
		case "#/components/parameters/TenantHeader":
			names["X-Stele-Tenant"] = true
		case "#/components/parameters/ProjectHeader":
			names["X-Stele-Project"] = true
		case "#/components/parameters/NamespaceHeader":
			names["X-Stele-Namespace"] = true
		}
	}
	if strings.HasPrefix(path, "/v1/admin/") {
		if !names["X-Stele-Admin-Key"] {
			t.Errorf("%s operation %s missing admin authentication parameter", path, op.OperationID)
		}
	} else if !names["X-Stele-API-Key"] {
		t.Errorf("%s operation %s missing API authentication parameter", path, op.OperationID)
	}
	for _, name := range []string{"X-Stele-Tenant", "X-Stele-Project", "X-Stele-Namespace"} {
		if path == "/v1/admin/jobs/governance/status" {
			break // this diagnostic endpoint intentionally uses configured default scope
		}
		if !names[name] {
			t.Errorf("%s operation %s missing exact-scope parameter %s", path, op.OperationID, name)
		}
	}
	hasClientError := false
	if op.Responses != nil {
		for code := range op.Responses.Map() {
			if strings.HasPrefix(code, "4") {
				hasClientError = true
				break
			}
		}
	}
	if !hasClientError && !strings.HasPrefix(path, "/v1/admin/ranking-rollouts") && !strings.HasPrefix(path, "/v1/admin/memory-quality/") {
		t.Errorf("%s operation %s missing bounded 4xx error response", path, op.OperationID)
	}
}

func operationForMethod(item *openapi3.PathItem, method string) *openapi3.Operation {
	switch method {
	case "get":
		return item.Get
	case "post":
		return item.Post
	case "put":
		return item.Put
	case "patch":
		return item.Patch
	case "delete":
		return item.Delete
	case "head":
		return item.Head
	case "options":
		return item.Options
	default:
		return nil
	}
}
