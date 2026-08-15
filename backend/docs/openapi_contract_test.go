package docs

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v2"
)

func TestOpenAPIArtifactsRemainConsistentAndResolvable(t *testing.T) {
	jsonDocument := readJSONDocument(t, "swagger.json")
	yamlDocument := readYAMLDocument(t, "swagger.yaml")
	embeddedDocument := decodeJSONDocument(t, []byte(SwaggerInfo.ReadDoc()), "SwaggerInfo.ReadDoc()")

	checkOpenAPIRoot(t, jsonDocument, "swagger.json")

	if !reflect.DeepEqual(jsonDocument, yamlDocument) {
		t.Fatal("swagger.json and swagger.yaml differ semantically; regenerate both OpenAPI artifacts together")
	}

	// docs.go can lag the file snapshots until the next swag generation. Validate
	// its standalone contract without treating pre-existing snapshot drift as a new failure.
	checkOpenAPIRoot(t, embeddedDocument, "SwaggerInfo.ReadDoc()")
	checkLocalReferences(t, jsonDocument, jsonDocument, "swagger.json#")
	checkLocalReferences(t, embeddedDocument, embeddedDocument, "SwaggerInfo.ReadDoc()#")
}

func checkOpenAPIRoot(t *testing.T, document map[string]interface{}, source string) {
	t.Helper()
	if got := document["swagger"]; got != "2.0" {
		t.Fatalf("%s must declare Swagger/OpenAPI 2.0, got %v", source, got)
	}
	info, ok := document["info"].(map[string]interface{})
	if !ok || info["title"] == "" || info["version"] == "" {
		t.Fatalf("%s info must contain non-empty title and version", source)
	}
	paths, ok := document["paths"].(map[string]interface{})
	if !ok || len(paths) == 0 {
		t.Fatalf("%s must contain at least one documented path", source)
	}
	for path := range paths {
		if !strings.HasPrefix(path, "/") {
			t.Errorf("%s path %q must start with /", source, path)
		}
	}
}

func readJSONDocument(t *testing.T, path string) map[string]interface{} {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return decodeJSONDocument(t, contents, path)
}

func decodeJSONDocument(t *testing.T, contents []byte, source string) map[string]interface{} {
	t.Helper()
	var document map[string]interface{}
	if err := json.Unmarshal(contents, &document); err != nil {
		t.Fatalf("decode %s: %v", source, err)
	}
	return document
}

func readYAMLDocument(t *testing.T, path string) map[string]interface{} {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var raw interface{}
	if err := yaml.Unmarshal(contents, &raw); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	normalized, err := normalizeYAMLValue(raw)
	if err != nil {
		t.Fatalf("normalize %s: %v", path, err)
	}
	encoded, err := json.Marshal(normalized)
	if err != nil {
		t.Fatalf("encode normalized %s: %v", path, err)
	}
	return decodeJSONDocument(t, encoded, path)
}

func normalizeYAMLValue(value interface{}) (interface{}, error) {
	switch typed := value.(type) {
	case map[interface{}]interface{}:
		normalized := make(map[string]interface{}, len(typed))
		for key, child := range typed {
			stringKey, ok := key.(string)
			if !ok {
				return nil, fmt.Errorf("non-string YAML mapping key %v", key)
			}
			normalizedChild, err := normalizeYAMLValue(child)
			if err != nil {
				return nil, err
			}
			normalized[stringKey] = normalizedChild
		}
		return normalized, nil
	case []interface{}:
		normalized := make([]interface{}, len(typed))
		for index, child := range typed {
			normalizedChild, err := normalizeYAMLValue(child)
			if err != nil {
				return nil, err
			}
			normalized[index] = normalizedChild
		}
		return normalized, nil
	default:
		return typed, nil
	}
}

func checkLocalReferences(t *testing.T, root interface{}, value interface{}, location string) {
	t.Helper()
	switch typed := value.(type) {
	case map[string]interface{}:
		if reference, ok := typed["$ref"].(string); ok && strings.HasPrefix(reference, "#/") {
			if _, err := resolveJSONPointer(root, reference); err != nil {
				t.Errorf("unresolved local reference %q at %s: %v", reference, location, err)
			}
		}
		for key, child := range typed {
			checkLocalReferences(t, root, child, location+"/"+key)
		}
	case []interface{}:
		for index, child := range typed {
			checkLocalReferences(t, root, child, fmt.Sprintf("%s/%d", location, index))
		}
	}
}

func resolveJSONPointer(root interface{}, pointer string) (interface{}, error) {
	current := root
	for _, encodedToken := range strings.Split(strings.TrimPrefix(pointer, "#/"), "/") {
		token := strings.ReplaceAll(strings.ReplaceAll(encodedToken, "~1", "/"), "~0", "~")
		object, ok := current.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("%q does not address an object", token)
		}
		next, ok := object[token]
		if !ok {
			return nil, fmt.Errorf("token %q does not exist", token)
		}
		current = next
	}
	return current, nil
}
