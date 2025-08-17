package domain

import (
	"context"

	"github.com/fpt/go-gennai-cli/pkg/message"
)

type ReAct interface {
	// Invoke sends a prompt to the ReAct model and returns the response
	Invoke(ctx context.Context, prompt string, thinkingChan chan<- string) (message.Message, error)
	GetLastMessage() message.Message
	ClearHistory()
	GetConversationSummary() string
}
