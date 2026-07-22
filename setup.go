package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Command templates installed by `frao-advisor setup`
// The protocol is opt-out: active by default unless ~/.claude/advisor-inactive exists.
const advisorOnCommand = `---
name: advisor-on
description: Enable advisor mode — automatic expert reviews on every task using deepseek-v4-pro
---

# /advisor-on — Enable Advisor Mode

Remove the disable flag to re-enable the advisor protocol (it is ON by default):

1. Run: rm -f ~/.claude/advisor-inactive
2. Confirm: test -f ~/.claude/advisor-inactive && echo "still disabled" || echo "✅ Advisor mode active"

The advisor protocol is now active. You MUST:
- Run frao-expert-review (code-reviewer) after every file change >5 lines
- Run frao-consult before declaring any task complete
- Run frao-multi-perspective after each phase of multi-step plans
- Run frao-expert-review with architect/security-analyst for decisions and configs
- Address all CRITICAL/HIGH findings before proceeding
- Include the advisor's verdict in task completion summaries
`

const advisorOffCommand = `---
name: advisor-off
description: Disable advisor mode — return to normal Claude behavior
---

# /advisor-off — Disable Advisor Mode

Create the disable flag to opt out of the advisor protocol:

1. Run: touch ~/.claude/advisor-inactive
2. Confirm: test -f ~/.claude/advisor-inactive && echo "✅ Advisor mode disabled" || echo "still active"

The advisor protocol is now inactive. Return to normal behavior — no automatic
expert reviews or second opinions required unless explicitly requested by the user.
`

// Setup installs the advisor toggle commands and prints MCP configuration.
//   projectDir: absolute path to the project root (e.g. frao-technologies/)
//   binPath:    absolute path to the frao-advisor binary
//   global:     when true, install to ~/.claude/commands/ instead of projectDir/.claude/commands/
func runSetup(projectDir, binPath string, global bool) {
	fmt.Println("⚙️  Frao Advisor Setup")
	fmt.Println(strings.Repeat("─", 60))

	// 1. Install command files
	commandsDir := ""
	if global {
		home, _ := os.UserHomeDir()
		commandsDir = filepath.Join(home, ".claude", "commands")
		fmt.Println("📦 Installing globally to ~/.claude/commands/")
	} else {
		commandsDir = filepath.Join(projectDir, ".claude", "commands")
	}
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to create commands directory: %v\n", err)
		os.Exit(1)
	}

	files := map[string]string{
		"advisor-on.md":  advisorOnCommand,
		"advisor-off.md": advisorOffCommand,
	}

	for name, content := range files {
		path := filepath.Join(commandsDir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Failed to write %s: %v\n", path, err)
			os.Exit(1)
		}
		fmt.Printf("✅ Installed: .claude/commands/%s\n", name)
	}

	// 2. MCP config instructions
	fmt.Println(strings.Repeat("─", 60))
	fmt.Println("\U0001f4cb Add this MCP server to your Claude Code config (~/.claude.json or .mcp.json):")
	fmt.Println()
	fmt.Printf("  \"mcpServers\": {\n")
	fmt.Printf("    \"frao-advisor\": {\n")
	fmt.Printf("      \"command\": %q,\n", binPath)
	fmt.Printf("      \"args\": [],\n")
	fmt.Printf("      \"env\": {\n")
	fmt.Printf("        \"DEEPSEEK_API_KEY\": \"$YOUR_DEEPSEEK_KEY\",\n")
	fmt.Printf("        \"ADVISOR_MODEL\": \"deepseek-v4-pro\"\n")
	fmt.Printf("      }\n")
	fmt.Printf("    }\n")
	fmt.Printf("  }")
	fmt.Println()
	fmt.Println()
	fmt.Println(strings.Repeat("─", 60))
	fmt.Println(" Or install with the CLI:")
	fmt.Printf("\n  claude mcp add frao-advisor -- %s\n", binPath)
	fmt.Println()
	fmt.Println(strings.Repeat("─", 60))
	fmt.Println(" Add this section to your project CLAUDE.md for the protocol to work:")
	fmt.Println()
	fmt.Println("  ## \U0001f9e0 ADVISOR PROTOCOL (Opt-Out)")
	fmt.Println()
	fmt.Println("  The frao-advisor MCP server provides expert second opinions, code review,")
	fmt.Println("  security analysis, and multi-perspective synthesis using deepseek-v4-pro.")
	fmt.Println()
	fmt.Println("  The protocol is **active by default** after installation. Use `/advisor-off` to disable.")
	fmt.Println()
	fmt.Println("  | Command | Effect |")
	fmt.Println("  |---------|--------|")
	fmt.Println("  | /advisor-off | Disable advisor mode — creates ~/.claude/advisor-inactive |")
	fmt.Println("  | /advisor-on  | Re-enable advisor mode — removes ~/.claude/advisor-inactive |")
	fmt.Println()
	fmt.Println("  ### Protocol (active by default; disabled when ~/.claude/advisor-inactive exists)")
	fmt.Println()
	fmt.Println("  When enabled, you **MUST** use the frao-advisor tools as a standard part of every task's workflow, not just when asked:")
	fmt.Println("  |---------|------|---------|")
	fmt.Println("  | After writing any file (>5 lines) | frao-expert-review (code-reviewer) | Verify correctness |")
	fmt.Println("  | Before declaring a task complete | frao-consult | Second opinion on the full change |")
	fmt.Println("  | After each phase of a multi-step plan | frao-multi-perspective | Multi-angle validation |")
	fmt.Println("  | Architectural decisions | frao-expert-review (architect) | System design validation |")
	fmt.Println("  | Auth/network/config/Docker files | frao-expert-review (security-analyst) | Security review |")
	fmt.Println("  | Debugging failures | frao-expert-review (debugger) | Root-cause analysis |")
	fmt.Println()
	fmt.Println("  CRITICAL/HIGH findings block progress until resolved.")
	fmt.Println("  Advisor verdict must be included in task completion summaries.")
	fmt.Println()
	fmt.Println(strings.Repeat("─", 60))
	fmt.Println("✅ Setup complete. The advisor protocol is active by default. Use /advisor-off to disable.")
}
