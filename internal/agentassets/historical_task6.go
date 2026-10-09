package agentassets

import _ "embed"

// historicalManifest21cd1c2 is the exact pre-polish manifest copied from the
// 21cd1c2 Task 6 bundle. Keep it trusted for force upgrades, but out of the
// copied bundle.
//
//go:embed testdata/21cd1c2/manifest.json
var historicalManifest21cd1c2 []byte
