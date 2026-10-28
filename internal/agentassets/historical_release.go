package agentassets

import _ "embed"

// historicalManifestV010 is the exact v0.1.0 manifest. Keep the released
// bundle trusted for force upgrades without treating its files as current.
//
//go:embed testdata/released-v0.1.0/manifest.json
var historicalManifestV010 []byte
