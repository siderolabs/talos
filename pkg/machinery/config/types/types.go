// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package types imports all configuration document types to register them.
package types

import (
	_ "github.com/siderolabs/talos/pkg/machinery/config/types/block"              // import config types to register them
	_ "github.com/siderolabs/talos/pkg/machinery/config/types/network"            // import config types to register them
	_ "github.com/siderolabs/talos/pkg/machinery/config/types/runtime"            // import config types to register them
	_ "github.com/siderolabs/talos/pkg/machinery/config/types/runtime/extensions" // import config types to register them
	_ "github.com/siderolabs/talos/pkg/machinery/config/types/security"           // import config types to register them
	_ "github.com/siderolabs/talos/pkg/machinery/config/types/siderolink"         // import config types to register them
	_ "github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"           // import config types to register them
)
