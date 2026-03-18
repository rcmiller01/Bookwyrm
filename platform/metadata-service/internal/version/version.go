package version

// Populated at build time via -ldflags:
//
//	go build -ldflags "-X metadata-service/internal/version.Version=0.18.0
//	  -X metadata-service/internal/version.Commit=$(git rev-parse --short HEAD)
//	  -X metadata-service/internal/version.BuildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)
