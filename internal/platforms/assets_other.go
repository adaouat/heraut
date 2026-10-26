//go:build !unix

package platforms

import "os"

const assetOpenFlags = os.O_RDONLY
