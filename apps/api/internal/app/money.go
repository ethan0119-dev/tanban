package app

// maxBusinessAmountCents is a deliberate domain limit, not a database limit.
// Keeping individual orders, refunds, manual adjustments and stored-value
// balances below ¥1,000,000 prevents nonsensical operator input and makes all
// intermediate arithmetic straightforward to audit.
const maxBusinessAmountCents int64 = 100_000_000

func nonNegativeSumWithin(limit int64, values ...int64) bool {
	if limit < 0 {
		return false
	}
	remaining := limit
	for _, value := range values {
		if value < 0 || value > remaining {
			return false
		}
		remaining -= value
	}
	return true
}
