package server

import (
	"os"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// The operations that list the apiToken security scheme in api/openapi.yaml must be
// exactly those isTokenOperation lets a token reach, so neither can change alone.
func TestTokenOperationsMatchSpec(t *testing.T) {
	raw, err := os.ReadFile("../../api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	type security []map[string][]string
	var spec struct {
		Security security                        `yaml:"security"`
		Paths    map[string]map[string]yaml.Node `yaml:"paths"`
	}
	if err := yaml.Unmarshal(raw, &spec); err != nil {
		t.Fatal(err)
	}
	var operations int
	for path, methods := range spec.Paths {
		for method, node := range methods {
			if !slices.Contains([]string{"get", "put", "post", "patch", "delete"}, method) {
				continue // path-level parameters
			}
			var op struct{ Security *security }
			if err := node.Decode(&op); err != nil {
				t.Fatalf("%s %s: %v", method, path, err)
			}
			operations++
			effective := spec.Security
			if op.Security != nil {
				effective = *op.Security
			}
			inSpec := slices.ContainsFunc(effective, func(s map[string][]string) bool { _, ok := s["apiToken"]; return ok })
			pattern := strings.ToUpper(method) + " /api/v1" + path
			if inSpec != isTokenOperation(pattern) {
				t.Errorf("%s: apiToken in spec = %v, isTokenOperation = %v", pattern, inSpec, !inSpec)
			}
		}
	}
	if operations < 10 {
		t.Fatalf("read only %d operations; is the spec layout what the test expects?", operations)
	}
}
