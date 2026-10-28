package pitstop

import _ "embed"

// InstallerScript is the checked-in release installer used by self-update.
//
//go:embed scripts/install.sh
var InstallerScript []byte
