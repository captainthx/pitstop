package agentassets

import _ "embed"

// historicalManifestBDEB767 is the exact pre-fix bundle manifest retained for
// force upgrades after the canonical Node manager bootstrap correction.
//
//go:embed testdata/bdeb767/manifest.json
var historicalManifestBDEB767 []byte
