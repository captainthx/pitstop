package agentassets

import _ "embed"

// historicalManifestD21a0af is the exact Task 7 bundle manifest. Keep it
// trusted for force upgrades while preserving its predecessor asset bytes in
// testdata/d21a0af.
//
//go:embed testdata/d21a0af/manifest.json
var historicalManifestD21a0af []byte
