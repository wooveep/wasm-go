package memorycache

const (
	// TrustedDigestProperty is private per-request filter state shared by gateway plugins.
	TrustedDigestProperty = "mse_ai_memory_context_digest"
	// DeprecatedDigestHeader is stripped so callers cannot spoof the private handoff.
	DeprecatedDigestHeader = "x-mse-memory-digest"
)
