package ckblp

const (
	policyFlagEnforceMaxFee = 1 << 0
	policyFlagEnforceMinFee = 1 << 1
	policyFlagRequirePrice  = 1 << 2
	policyFlagSafePrice     = 1 << 3

	// PolicyFlagRequirePrice / PolicyFlagSafePrice expose the policy flag bits
	// callers need to assert on LP cell config (e.g. tests that pass priceX64=1
	// and need to know neither flag is set).
	PolicyFlagRequirePrice = policyFlagRequirePrice
	PolicyFlagSafePrice    = policyFlagSafePrice
)
