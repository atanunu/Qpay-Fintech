package httpapi

import (
	"github.com/atanunu/Qpay-Fintech/APIbackend/internal/service"
	"testing"
)

func TestOpenAPIQueryCSVAndMessageShapes(t *testing.T) {
	h := New(&service.Service{}, Config{})
	paths := h.OpenAPI()["paths"].(map[string]any)
	operation := func(path, method string) map[string]any { return paths[path].(map[string]any)[method].(map[string]any) }
	response := func(op map[string]any, code string) map[string]any {
		return op["responses"].(map[string]any)[code].(map[string]any)
	}
	statement := operation("/v1/statements", "get")
	if len(statement["parameters"].([]any)) != 3 {
		t.Fatal("missing statement query parameters")
	}
	if response(statement, "200")["content"].(map[string]any)["text/csv"] == nil {
		t.Fatal("missing CSV contract")
	}
	message := response(operation("/v1/support/cases/{id}/messages", "post"), "201")["content"].(map[string]any)["application/json"].(map[string]any)["schema"].(map[string]any)["properties"].(map[string]any)["data"].(map[string]any)
	if message["properties"].(map[string]any)["status"] == nil {
		t.Fatal("reply contract does not match accepted status")
	}
	noContent := response(operation("/v1/beneficiaries/{id}", "delete"), "204")
	if noContent["content"] != nil {
		t.Fatal("204 must not declare a JSON response body")
	}
}
