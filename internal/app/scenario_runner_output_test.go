package app

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/fpt/go-gennai-cli/internal/config"
	"github.com/fpt/go-gennai-cli/internal/infra"
	"github.com/fpt/go-gennai-cli/pkg/agent/domain"
	pkgLogger "github.com/fpt/go-gennai-cli/pkg/logger"
	"github.com/fpt/go-gennai-cli/pkg/message"
)

// mockToolLLM is a minimal ToolCallingLLM that emits thinking content and returns a simple response.
type mockToolLLM struct{}

func (m *mockToolLLM) Chat(ctx context.Context, messages []message.Message, enableThinking bool, thinkingChan chan<- string) (message.Message, error) {
	// Emit some thinking content then signal end
	message.SendThinkingContent(thinkingChan, "thinking-1 ")
	message.SendThinkingContent(thinkingChan, "thinking-2")
	message.EndThinking(thinkingChan)
	// Return a simple assistant message
	return message.NewChatMessage(message.MessageTypeAssistant, "ok"), nil
}

func (m *mockToolLLM) SetToolManager(_ domain.ToolManager) {}

func (m *mockToolLLM) ChatWithToolChoice(ctx context.Context, messages []message.Message, toolChoice domain.ToolChoice, enableThinking bool, thinkingChan chan<- string) (message.Message, error) {
	// Delegate to Chat for simplicity
	return m.Chat(ctx, messages, enableThinking, thinkingChan)
}

func (m *mockToolLLM) ModelID() string { return "mock-llm" }

// Test that NewScenarioRunnerWithOptions uses the provided io.Writer for thinking output.
func TestScenarioRunner_OutputWriter_Thinking(t *testing.T) {
	var buf bytes.Buffer

	// Route console logs to the same writer to avoid stdio noise in tests
	pkgLogger.SetGlobalLoggerWithConsoleWriter(pkgLogger.LogLevelDebug, &buf)
	logger := pkgLogger.NewLoggerWithConsoleWriter(pkgLogger.LogLevelDebug, &buf)

	llm := &mockToolLLM{}
	settings := config.GetDefaultSettings()

	fsRepo := infra.NewOSFilesystemRepository()
	runner := NewScenarioRunnerWithOptions(llm, ".", map[string]domain.ToolManager{}, settings, logger, &buf, true, true, fsRepo)

	// Invoke using universal tools path (avoids scenario YAMLs)
	_, err := runner.InvokeWithOptions(context.Background(), "hello")
	if err != nil {
		t.Fatalf("InvokeWithOptions error: %v", err)
	}

	// Yield to the thinking printer goroutine
	// (it writes to the provided writer asynchronously)
	// A tiny delay is enough since our mock ends thinking immediately
	// and writing is buffered in-memory.
	// In case of flakes on slower CI, increase this value slightly.
	//nolint:gomnd
	time.Sleep(20 * time.Millisecond)

	out := buf.String()
	// Expect the gray-thinking prefix emoji and our content pieces to appear
	if !strings.Contains(out, "💭") {
		t.Fatalf("expected thinking emoji in output, got: %q", out)
	}
	if !strings.Contains(out, "thinking-1") || !strings.Contains(out, "thinking-2") {
		t.Fatalf("expected thinking content in output, got: %q", out)
	}
}
