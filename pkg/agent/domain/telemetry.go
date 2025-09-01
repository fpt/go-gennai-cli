package domain

import (
	"github.com/fpt/go-gennai-cli/pkg/message"
)

// TokenUsageProvider is an optional extension that LLM clients can implement
// to expose token accounting information from the most recent API call.
//
// Implementations should return (usage, true) when token usage was available
// for the last Chat/ChatWithToolChoice/ChatWithStructure invocation, and
// (message.TokenUsage{}, false) if unavailable.
//
// Callers should treat this as a best-effort signal and not rely on it for
// strict billing; backends may omit or delay usage reporting.
type TokenUsageProvider interface {
	LastTokenUsage() (message.TokenUsage, bool)
}

// ModelIdentifier is an optional extension that clients can implement to
// return a stable identifier for the underlying model. This can be used for
// telemetry and to compose cache keys.
type ModelIdentifier interface {
	ModelID() string
}
