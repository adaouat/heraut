package config

import forgeconfig "github.com/adaouat/forge/config"

// ValidationError and ValidationErrors are heraut's aliases for forge's
// structured validation errors (Path/Message/Hint). The validator produces
// them; cmd renders them.
type (
	ValidationError  = forgeconfig.ValidationError
	ValidationErrors = forgeconfig.ValidationErrors
)
