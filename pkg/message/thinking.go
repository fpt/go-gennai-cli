package message

import (
	"fmt"
	"os"
)

// ThinkingEvent represents different types of thinking events
type ThinkingEvent struct {
	Type    ThinkingEventType
	Content string
}

// ThinkingEventType defines the type of thinking event
type ThinkingEventType int

const (
	ThinkingStart ThinkingEventType = iota
	ThinkingContent
	ThinkingEnd
)

// ThinkingPrinter handles centralized printing of thinking messages using channels
type ThinkingPrinter struct {
	channel chan string
	started bool
}

// NewThinkingPrinter creates a new thinking printer that listens on the provided channel
func NewThinkingPrinter(channel chan string) *ThinkingPrinter {
	return &ThinkingPrinter{
		channel: channel,
		started: false,
	}
}

// StartListening starts listening for thinking content on the channel and prints it
// This should be called in a goroutine as it blocks until the channel is closed
func (tp *ThinkingPrinter) StartListening() {
	for content := range tp.channel {
		if content == "" {
			// Empty string signals end of thinking
			if tp.started {
				fmt.Print("\x1b[0m\n") // Reset color and newline
				os.Stdout.Sync()
				tp.started = false
			}
		} else {
			// First content triggers header
			if !tp.started {
				fmt.Print("\x1b[90m💭 ") // Gray thinking emoji
				tp.started = true
			}

			// Print content in gray without reset
			fmt.Printf("\x1b[90m%s", content)
			os.Stdout.Sync()
		}
	}
}

// CreateThinkingChannel creates a new thinking channel and starts a printer goroutine
// Returns the send-only channel for clients to use
func CreateThinkingChannel() chan<- string {
	thinkingChan := make(chan string, 100) // Buffered to prevent blocking

	// Start printer goroutine
	go func() {
		printer := NewThinkingPrinter(thinkingChan)
		printer.StartListening()
	}()

	return thinkingChan
}

// SendThinkingContent sends thinking content to the channel
// Use empty string to signal end of thinking
func SendThinkingContent(channel chan<- string, content string) {
	if channel != nil {
		select {
		case channel <- content:
			// Content sent successfully
		default:
			// Channel is full, drop the content to prevent blocking
			// This ensures streaming doesn't block even if channel is overwhelmed
		}
	}
}

// EndThinking signals the end of thinking by sending empty string
func EndThinking(channel chan<- string) {
	SendThinkingContent(channel, "")
}
