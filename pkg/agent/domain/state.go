package domain

import (
	"context"

	"github.com/fpt/go-gennai-cli/pkg/message"
)

type State interface {
	GetMessages() []message.Message
	AddMessage(msg message.Message)
	GetLastMessage() message.Message
	Clear()
	// CleanupMandatory performs mandatory cleanup (remove images, aligner messages) without compaction
	CleanupMandatory() error
	// CompactIfNeeded performs compaction only if token usage exceeds threshold
	CompactIfNeeded(ctx context.Context, llm LLM, maxTokens int, thresholdPercent float64) error
	GetValidConversationHistory(maxMessages int) []message.Message
	RemoveMessagesBySource(source message.MessageSource) int
	// GetTotalTokenUsage returns the total token usage across all messages
	GetTotalTokenUsage() (inputTokens, outputTokens, totalTokens int)
	// Context persistence
	Serialize() ([]byte, error)
	Deserialize(data []byte) error
	SaveToFile(filePath string) error
	LoadFromFile(filePath string) error
}
