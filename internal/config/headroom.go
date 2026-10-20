package config

// Missing configuration enables the installed system plugin whenever AI is
// enabled. An explicit false remains an operator override.
func (c AIConfig) UseHeadroom() bool {
	return c.Enabled && (c.HeadroomEnabled == nil || *c.HeadroomEnabled)
}
