package app

import (
	"fmt"
	"strings"
	"time"
)

const pasteInterval = time.Millisecond * 10
const initialPasteWindow = time.Millisecond * 25
const minPasteBlockLen = 100 // only treat as paste if block >= 100 chars

// PromptBuilder is a minimal wrapper around user input.
// First step: it simply accumulates runes and returns the same
// string for both VisiblePrompt and RawPrompt.
type PromptBuilder struct {
	buf   []rune
	times []time.Time
}

// NewPromptBuilder creates a new PromptBuilder.
func NewPromptBuilder() *PromptBuilder {
	return &PromptBuilder{
		buf:   make([]rune, 0, 256),
		times: make([]time.Time, 0, 256),
	}
}

// Input appends a single character to the prompt buffer.
func (p *PromptBuilder) Input(r rune) {
	now := time.Now()
	p.buf = append(p.buf, r)
	p.times = append(p.times, now)
}

// VisiblePrompt returns the string to show in the UI.
func (p *PromptBuilder) VisiblePrompt() string {
	if len(p.buf) == 0 {
		return ""
	}
	// Replace fast-typed (pasted) contiguous regions with a short placeholder.
	out := make([]rune, 0, len(p.buf))
	i := 0
	for i < len(p.buf) {
		if i > 0 && p.times[i].Sub(p.times[i-1]) < pasteInterval {
			// find left boundary
			start := i - 1
			for start > 0 && p.times[start].Sub(p.times[start-1]) < pasteInterval {
				start--
			}
			// find right boundary
			end := i
			for end+1 < len(p.buf) && p.times[end+1].Sub(p.times[end]) < pasteInterval {
				end++
			}
			// If paste burst started at the very beginning within a small window,
			// include the first character as part of the paste block.
			if start == 1 && p.times[end].Sub(p.times[0]) <= initialPasteWindow {
				start = 0
			}

			n := end - start + 1
			if n >= minPasteBlockLen {
				// Build preview (first 20 runes)
				run := p.buf[start : end+1]
				previewLen := 20
				if n < previewLen {
					previewLen = n
				}
				preview := string(run[:previewLen])
				ellipsis := ""
				if n > previewLen {
					ellipsis = "…"
				}
				placeholder := []rune(fmt.Sprintf("[pasted: '%s%s', %d chars]", preview, ellipsis, n))
				out = append(out, placeholder...)
				i = end + 1
				continue
			}
			// If below threshold, do not compress; fall through to append current rune normally.
		}
		r := p.buf[i]
		if r == '\n' || r == '\r' {
			r = ' '
		}
		out = append(out, r)
		i++
	}
	return string(out)
}

// RawPrompt returns the string to send to the model/tools.
func (p *PromptBuilder) RawPrompt() string {
	return string(p.buf)
}

// IsSlashCommand reports whether the raw buffer (trimmed) starts with '/'.
// Uses the unmodified buffer so detection is not affected by paste-compression
// used in VisiblePrompt().
func (p *PromptBuilder) IsSlashCommand() bool {
	s := strings.TrimSpace(string(p.buf))
	return strings.HasPrefix(s, "/")
}

// SlashInput returns the trimmed raw buffer when it represents a slash command.
// Returns an empty string if it's not a slash command.
func (p *PromptBuilder) SlashInput() string {
	s := strings.TrimSpace(string(p.buf))
	if strings.HasPrefix(s, "/") {
		return s
	}
	return ""
}

// Backspace removes the last rune if present.
func (p *PromptBuilder) Backspace() {
	n := len(p.buf)
	if n == 0 {
		return
	}
	// If the last two runes were entered within pasteInterval, treat the
	// trailing contiguous fast-typed region as a single unit and delete it.
	if n >= 2 && p.times[n-1].Sub(p.times[n-2]) < pasteInterval {
		// Find left boundary (matching VisiblePrompt compression rule)
		start := n - 2
		for start > 0 && p.times[start].Sub(p.times[start-1]) < pasteInterval {
			start--
		}
		// If the burst began near the start and stayed within the initial window, include first char.
		if start == 1 && p.times[n-1].Sub(p.times[0]) <= initialPasteWindow {
			start = 0
		}
		blockLen := n - start
		if blockLen >= minPasteBlockLen {
			// Delete [start..n-1]
			p.buf = append(p.buf[:start], p.buf[n:]...)
			p.times = append(p.times[:start], p.times[n:]...)
			return
		}
	}
	// Otherwise, delete one rune
	p.buf = p.buf[:n-1]
	if len(p.times) > 0 {
		p.times = p.times[:len(p.times)-1]
	}
}

// Clear resets the buffer.
func (p *PromptBuilder) Clear() {
	p.buf = p.buf[:0]
	p.times = p.times[:0]
}

// Len returns the current rune length.
func (p *PromptBuilder) Len() int { return len(p.buf) }
