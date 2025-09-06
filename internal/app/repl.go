package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/chzyer/readline"
	"github.com/fpt/go-gennai-cli/pkg/agent/domain"
	"github.com/manifoldco/promptui"
)

// SlashCommand represents a command that starts with /
type SlashCommand struct {
	Name        string
	Description string
	Handler     func(*ScenarioRunner) bool // Returns true if should exit
}

// getSlashCommands returns all available slash commands
func getSlashCommands() []SlashCommand {
	return []SlashCommand{
		{
			Name:        "help",
			Description: "Show available commands and usage information",
			Handler: func(a *ScenarioRunner) bool {
				showInteractiveHelp()
				return false
			},
		},
		{
			Name:        "log",
			Description: "Show conversation history (preview)",
			Handler: func(a *ScenarioRunner) bool {
				history := a.GetConversationPreview(1000)
				if strings.TrimSpace(history) == "" {
					fmt.Println("📜 No conversation history found.")
					return false
				}
				fmt.Println(history)
				return false
			},
		},
		{
			Name:        "clear",
			Description: "Clear conversation history and start fresh",
			Handler: func(a *ScenarioRunner) bool {
				a.ClearHistory()
				fmt.Println("🧹 Conversation history cleared.")
				return false
			},
		},
		{
			Name:        "status",
			Description: "Show current session status and statistics",
			Handler: func(a *ScenarioRunner) bool {
				showStatus(a)
				return false
			},
		},
		{
			Name:        "quit",
			Description: "Exit the interactive session",
			Handler: func(a *ScenarioRunner) bool {
				fmt.Println("👋 Goodbye!")
				return true
			},
		},
		{
			Name:        "exit",
			Description: "Exit the interactive session (alias for quit)",
			Handler: func(a *ScenarioRunner) bool {
				fmt.Println("👋 Goodbye!")
				return true
			},
		},
	}
}

// handleSlashCommand processes commands that start with /
// Returns true if the command requests program exit, false otherwise
func handleSlashCommand(input string, a *ScenarioRunner) bool {
	// Check if this is just "/" - show command selector
	if strings.TrimSpace(input) == "/" {
		return showCommandSelector(a)
	}

	parts := strings.Fields(input)
	if len(parts) == 0 {
		return false
	}

	commandName := strings.TrimPrefix(parts[0], "/")
	commands := getSlashCommands()

	// Find and execute the command
	for _, cmd := range commands {
		if cmd.Name == commandName {
			return cmd.Handler(a)
		}
	}

	// Command not found - show available commands
	fmt.Printf("❌ Unknown command: /%s\n", commandName)
	fmt.Println("💡 Available commands:")
	for _, cmd := range commands {
		fmt.Printf("  /%s - %s\n", cmd.Name, cmd.Description)
	}
	fmt.Println("\n💡 Tip: Type just '/' to see an interactive command selector!")
	return false
}

// showCommandSelector shows an interactive command selector using promptui
func showCommandSelector(a *ScenarioRunner) bool {
	commands := getSlashCommands()

	templates := &promptui.SelectTemplates{
		Label:    "{{ . }}?",
		Active:   "▸ {{ .Name | cyan }} - {{ .Description | faint }}",
		Inactive: "  {{ .Name | cyan }} - {{ .Description | faint }}",
		Selected: "{{ .Name | red | cyan }}",
		Details: `
--------- Command Details ----------
{{ "Name:" | faint }}\t{{ .Name }}
{{ "Description:" | faint }}\t{{ .Description }}`,
	}

	searcher := func(input string, index int) bool {
		command := commands[index]
		name := strings.ReplaceAll(strings.ToLower(command.Name), " ", "")
		input = strings.ReplaceAll(strings.ToLower(input), " ", "")
		return strings.Contains(name, input)
	}

	prompt := promptui.Select{
		Label:     "Choose a command",
		Items:     commands,
		Templates: templates,
		Size:      10,
		Searcher:  searcher,
	}

	i, _, err := prompt.Run()
	if err != nil {
		if err == promptui.ErrInterrupt {
			fmt.Println("\nCancelled.")
			return false
		}
		fmt.Printf("Command selection failed: %v\n", err)
		return false
	}
	return commands[i].Handler(a)
}

// StartInteractiveMode runs the readline-based REPL
func StartInteractiveMode(ctx context.Context, a *ScenarioRunner, scenario string) {
	// Configure readline with enhanced features
	// Context display
	contextDisplay := NewContextDisplay()

	// Use a long-lived PromptBuilder for this readline session
	pb := NewPromptBuilder(a.FilesystemRepository(), a.WorkingDir())

	rlCfg := &readline.Config{
		Prompt:                 "> ",
		HistoryFile:            "",
		AutoComplete:           createAutoCompleter(),
		InterruptPrompt:        "^C",
		EOFPrompt:              "exit",
		HistorySearchFold:      true,
		HistoryLimit:           2000,
		DisableAutoSaveHistory: true,
		FuncFilterInputRune:    filterInput,
	}

	// Intercept key events; record printable runes and handle backspace at EOL.
	rlCfg.SetListener(func(line []rune, pos int, key rune) (newLine []rune, newPos int, ok bool) {
		// Ctrl+C: allow readline to handle as interrupt
		if key == 3 { // Ctrl+C
			return nil, 0, false
		}
		// Ctrl+A: move to beginning; keep builder unchanged but update cursor.
		if key == 1 { // Ctrl+A
			vis := []rune(pb.VisiblePrompt())
			return vis, 0, true
		}
		// Ctrl+K: kill to end of line. If at start, clear builder and line.
		if key == 11 { // Ctrl+K
			if pos == 0 {
				pb.Clear()
				return []rune{}, 0, true
			}
			// For mid-line positions we defer to readline; builder may diverge until submit.
			return nil, 0, false
		}
		// TODO: Newline from paste: capture but do not submit
		// if key == '\n' || key == '\r' {
		// 	// Treat as part of content (likely from paste)
		// 	pb.Input(key)
		// 	vis := []rune(pb.VisiblePrompt())
		// 	return vis, len(vis), true
		// }
		// Printable runes: append only when typing at end of line
		if key >= 0x20 && key != 0x7f { // exclude DEL
			if pos == len(line) {
				pb.Input(key)
			}
			visiblePrompt := pb.VisiblePrompt()
			vis := []rune(visiblePrompt)
			return vis, len(vis), true
		}
		// Backspace (common codes: 127=DEL, 8=BS) when at end of line
		if key == 127 || key == 8 {
			if pos == len(line) {
				pb.Backspace()
			}
			visiblePrompt := pb.VisiblePrompt()
			vis := []rune(visiblePrompt)
			return vis, len(vis), true
		}
		// Ignore navigation/other control keys; do not replace the line
		return nil, 0, false
	})

	rl, err := readline.NewEx(rlCfg)
	if err != nil {
		fmt.Printf("❌ Failed to initialize interactive mode: %v\n", err)
		fmt.Println("💡 Please use one-shot mode instead: gennai \"your request here\"")
		return
	}
	defer rl.Close()

	// Detect model ID if available
	modelID := "unknown"
	if mi, ok := a.llmClient.(domain.ModelIdentifier); ok {
		modelID = mi.ModelID()
	}

	// Optional splash screen
	WriteSplashScreen(os.Stdout, true)
	fmt.Printf("🧠 Model: %s\n", modelID)
	fmt.Println("💬 Commands start with '/', everything else goes to the AI agent!")
	fmt.Println("⌨️ Arrow keys to navigate; Tab for completion; Ctrl+R searches this session's input.")
	fmt.Println(strings.Repeat("=", 60))

	if preview := a.GetConversationPreview(6); preview != "" {
		fmt.Print("\n")
		fmt.Print(preview)
		fmt.Println()
	}

	for {
		pb.Clear() // Clear the prompt buffer at the start of each loop

		// Show context usage above the prompt, reflecting the latest LLM turn
		line := contextDisplay.ShowContextUsage(a.GetMessageState(), a.GetLLMClient())
		if line != "" {
			fmt.Printf("%s\n", line)
		}

		line, err := rl.Readline()
		if err == readline.ErrInterrupt {
			if len(line) == 0 {
				break
			}
			continue
		} else if err == io.EOF {
			break
		}

		// Handle slash commands using the raw buffer so paste compression
		// in VisiblePrompt() does not interfere with detection.
		if pb.IsSlashCommand() {
			cmd := pb.SlashInput()
			if handleSlashCommand(cmd, a) {
				break
			}

			// Clear and refresh
			pb.Clear()
			rl.Clean()
			rl.Refresh()
			continue
		}

		// Use builder-captured view for display (may compress pastes)
		userInput := pb.VisiblePrompt()

		// @filename processing is now handled automatically by PromptBuilder
		// VisiblePrompt shows highlights, RawPrompt embeds file content

		if userInput == "" {
			continue
		}

		// Execute via scenario runner with cancellable context
		// Set up signal handling for Ctrl+C during execution
		execCtx, cancel := context.WithCancel(ctx)
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT)

		// Handle Ctrl+C during execution in a goroutine
		go func() {
			select {
			case <-sigChan:
				fmt.Println() // Move to new line after ^C
				cancel()      // Cancel the execution context
			case <-execCtx.Done():
				// Execution finished, clean up
			}
		}()

		response, invokeErr := a.Invoke(execCtx, pb.RawPrompt(), scenario)

		// Check for cancellation BEFORE cleaning up
		wasCanceled := execCtx.Err() == context.Canceled

		// Clean up signal handling
		signal.Stop(sigChan)
		close(sigChan)
		cancel()

		if invokeErr != nil {
			// Check if the error was due to cancellation
			if wasCanceled {
				fmt.Printf("🔄 Ready for next command.\n")
			} else {
				fmt.Printf("❌ Error: %v\n", invokeErr)
			}
			continue
		}
		// Print response via ScenarioRunner's writer with model header
		w := a.OutWriter()
		model := a.GetLLMClient().ModelID()
		// Skyblue/bright-cyan header without icon
		WriteResponseHeader(w, model, true)
		fmt.Fprintln(w, response.Content())

		// No placeholder state to reset
	}
}

// createAutoCompleter creates an autocompletion function for readline
func createAutoCompleter() *readline.PrefixCompleter {
	commands := getSlashCommands()
	var pcItems []readline.PrefixCompleterInterface
	for _, cmd := range commands {
		pcItems = append(pcItems, readline.PcItem("/"+cmd.Name))
	}
	pcItems = append(pcItems, readline.PcItem("/"))
	for _, pattern := range []string{
		"Create a", "Analyze the", "Write unit tests for", "List files in",
		"Run go build", "Fix any errors", "Explain how", "Show me",
		"Generate", "Debug", "Test", "Refactor",
	} {
		pcItems = append(pcItems, readline.PcItem(pattern))
	}
	return readline.NewPrefixCompleter(pcItems...)
}

// filterInput filters input runes to handle special keys
func filterInput(r rune) (rune, bool) {
	switch r {
	case readline.CharCtrlZ:
		return r, false
	}
	return r, true
}

func showInteractiveHelp() {
	commands := getSlashCommands()
	fmt.Println("\n📚 Interactive Commands:")
	fmt.Println("  /                - Show interactive command selector 🆕")
	for _, cmd := range commands {
		fmt.Printf("  /%-15s - %s\n", cmd.Name, cmd.Description)
	}
	fmt.Println("\n⌨️  Enhanced Features:")
	fmt.Println("  Ctrl+C           - Cancel current input")
	fmt.Println("  Ctrl+R           - Search this session's input history")
	fmt.Println("  Tab              - Auto-complete commands and patterns")
	fmt.Println("  Arrow keys       - Navigate input and history")
	fmt.Println("  /                - Interactive command selector with search!")
	fmt.Println("\n💡 Example requests:")
	fmt.Println("  > Create a HTTP server with health check")
	fmt.Println("  > Analyze the current codebase structure")
	fmt.Println("  > Write unit tests for the ScenarioRunner")
	fmt.Println("  > List files in the current directory")
	fmt.Println("  > Run go build and fix any errors")
	fmt.Println("\n✨ New: Type just '/' to see a beautiful command selector!")
	fmt.Println("🔧 The agent will automatically use tools when needed!")
}

func showStatus(a *ScenarioRunner) {
	fmt.Println("\n📊 Session Status:")
	preview := a.GetConversationPreview(100)
	if preview != "" {
		userMsgCount := strings.Count(preview, "👤 You:")
		assistantMsgCount := strings.Count(preview, "🤖 Assistant:")
		fmt.Printf("  💬 Messages: %d from you, %d from assistant\n", userMsgCount, assistantMsgCount)
	} else {
		fmt.Println("  💬 Messages: No conversation history")
	}
	fmt.Println("  🔧 Tools: Available and active")
	fmt.Println("  🧠 Agent: ReAct with scenario planning")
	fmt.Println("  ⚡ Status: Ready for requests")
}
