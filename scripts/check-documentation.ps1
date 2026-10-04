$ErrorActionPreference = 'Stop'

go test ./internal/docscheck ./docs -count=1
