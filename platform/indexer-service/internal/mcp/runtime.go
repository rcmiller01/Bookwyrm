package mcp

import (
	"os"
	"strings"

	"indexer-service/internal/indexer"
)

type Runtime struct{}

func NewRuntime() *Runtime {
	return &Runtime{}
}

func (r *Runtime) HeadersFor(server indexer.MCPServerRecord) map[string]string {
	headers := map[string]string{}
	for headerName, envVar := range server.EnvMapping {
		headerName = strings.TrimSpace(headerName)
		envVar = strings.TrimSpace(envVar)
		if headerName == "" || envVar == "" {
			continue
		}
		if value := strings.TrimSpace(os.Getenv(envVar)); value != "" {
			headers[headerName] = value
		}
	}
	return headers
}
