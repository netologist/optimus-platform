package transport

import (
	_ "embed"
	"os"
)

//go:embed openapi.json
var defaultOpenAPISpecJSON []byte

//go:embed swagger.html
var defaultSwaggerUIHTML []byte

// GetOpenAPISpec returns the OpenAPI 3.0.3 specification in JSON format.
// If OPENAPI_SPEC_PATH environment variable is set and readable, it loads from disk;
// otherwise it returns the compiled embedded default specification.
func GetOpenAPISpec() []byte {
	if customPath := os.Getenv("OPENAPI_SPEC_PATH"); customPath != "" {
		if data, err := os.ReadFile(customPath); err == nil {
			return data
		}
	}
	return defaultOpenAPISpecJSON
}

// GetSwaggerUIHTML returns the HTML page hosting Swagger UI.
func GetSwaggerUIHTML() []byte {
	return defaultSwaggerUIHTML
}
