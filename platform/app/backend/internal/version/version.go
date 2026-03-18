package version

// Populated at build time via -ldflags:
//
//	go build -ldflags "-X app-backend/internal/version.Version=0.18.0
//	  -X app-backend/internal/version.Commit=$(git rev-parse --short HEAD)
//	  -X app-backend/internal/version.BuildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)
