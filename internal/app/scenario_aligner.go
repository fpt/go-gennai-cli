package app

import (
	"fmt"
	"strings"

	"github.com/fpt/go-gennai-cli/internal/tool"
	"github.com/fpt/go-gennai-cli/pkg/agent/domain"
	pkgLogger "github.com/fpt/go-gennai-cli/pkg/logger"
	"github.com/fpt/go-gennai-cli/pkg/message"
)

// Package-level logger for scenario alignment operations
var logger = pkgLogger.NewComponentLogger("scenario-aligner")

type ScenarioAligner struct {
	todoToolManager *tool.TodoToolManager
}

func NewScenarioAligner(todoToolManager *tool.TodoToolManager) *ScenarioAligner {
	return &ScenarioAligner{
		todoToolManager: todoToolManager,
	}
}

func (s *ScenarioAligner) InjectMessage(state domain.State, curIter, iterLimit int) {
	// Shortcut for last iteration message
	if curIter >= iterLimit-1 {
		systemMessage := fmt.Sprintf("IMPORTANT: This is iteration %d/%d. Conclude your response based on the knowledge so far.",
			curIter, iterLimit)
		state.AddMessage(message.NewAlignerSystemMessage(systemMessage))
		return
	}

	var messages []string

	// if the last message is a tool response, we prepend a special system message
	if lastMsg := state.GetLastMessage(); lastMsg != nil && lastMsg.Type() == message.MessageTypeToolResult {
		logger.DebugWithIcon("🔧", "Found tool result, prepending system message")

		// Create more specific system message based on tool result content
		if len(lastMsg.Images()) > 0 {
			// Special handling for image results - emphasize visual analysis
			messages = append(messages, "You received a tool result with visual content (images). IMPORTANT: You must analyze the images and provide a comprehensive visual analysis based on what you can see in the images. Focus on the user's original request and describe the visual content thoroughly. Do not call additional tools - provide your final analysis based on the visual information.")
		} else {
			// Regular tool result
			messages = append(messages, "You received a tool result. Analyze it and decide next steps to respond to original user request.")
		}
	}

	todosContext := s.todoToolManager.GetTodosForPrompt()
	if todosContext != "" {
		messages = append(messages, fmt.Sprintf("## Current Todos:\n%s\n\nConsider these todos when responding and use TodoWrite tool to update progress.",
			todosContext))

		logger.DebugWithIcon("📋", "Enriched user message with todo context", "context_length", len(todosContext))
	}

	if len(messages) > 0 {
		systemMessage := strings.Join(messages, "\n")
		state.AddMessage(message.NewAlignerSystemMessage(systemMessage))
	}
}
