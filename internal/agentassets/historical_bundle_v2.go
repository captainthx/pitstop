package agentassets

import _ "embed"

// historicalManifestV2 is the exact predecessor manifest retained for force upgrades.
// Its changed payload bytes live under testdata/bundle-v2 for regression checks.
//
//go:embed testdata/bundle-v2/manifest.json
var historicalManifestV2 []byte
