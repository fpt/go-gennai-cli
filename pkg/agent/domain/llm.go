package domain

import (
	"context"
	"errors"

	"github.com/fpt/go-gennai-cli/pkg/message"
)

var ErrInvalidClientType = errors.New("invalid client type for tool calling")

// LLM represents the base language model interface for basic chat functionality
type LLM interface {
	// Chat sends a message to the LLM and returns the response
	Chat(ctx context.Context, messages []message.Message, enableThinking bool) (message.Message, error)
}

// ToolCallingLLM extends LLM with tool calling capabilities
type ToolCallingLLM interface {
	LLM

	// SetToolManager sets the tool manager for this client
	SetToolManager(toolManager ToolManager)

	// ChatWithToolChoice sends a message to the LLM with tool choice control
	ChatWithToolChoice(ctx context.Context, messages []message.Message, toolChoice ToolChoice) (message.Message, error)
}

// VisionLLM extends LLM with vision capabilities for image analysis
type VisionLLM interface {
	LLM

	// SupportsVision returns true if this client supports vision/image analysis
	SupportsVision() bool
}
