package capture

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"

	"github.com/sirupsen/logrus"
)

// Format specifies the output format.
type Format string

const (
	FormatMarkdown Format = "markdown"
	FormatHTML     Format = "html"
)

// Options configures the capture behavior.
type Options struct {
	MaxDepth        int
	Format          Format
	SubcommandOrder []string // Priority order for subcommands (rest alphabetical)
}

// Capturer recursively captures help output from CLI tools.
type Capturer struct {
	logger *logrus.Logger
}

// New creates a new Capturer instance.
func New(logger *logrus.Logger) *Capturer {
	return &Capturer{logger: logger}
}

// CommandNode represents a command and its subcommands.
type CommandNode struct {
	Name        string
	FullName    string // e.g. "nb concept new"
	HelpOutput  string // Plain text (ANSI stripped)
	RawOutput   string // Raw output with ANSI codes
	SubCommands []*CommandNode
}

// Capture crawls a binary's help output and generates documentation.
func (c *Capturer) Capture(binaryPath, outputPath string, opts Options) error {
	if opts.Format == "" {
		opts.Format = FormatMarkdown
	}

	root := &CommandNode{
		Name:     binaryPath,
		FullName: binaryPath,
	}

	c.logger.Infof("Crawling %s...", binaryPath)
	forceColor := opts.Format == FormatHTML
	if err := c.crawl(root, 0, opts.MaxDepth, forceColor); err != nil {
		return err
	}

	// Sort subcommands based on priority order
	if len(opts.SubcommandOrder) > 0 {
		c.sortSubcommands(root, opts.SubcommandOrder)
	}

	c.logger.Info("Rendering documentation...")
	var content string
	switch opts.Format {
	case FormatHTML:
		content = c.renderHTML(root)
	default:
		content = c.render(root)
	}

	if err := os.WriteFile(outputPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	return nil
}

func (c *Capturer) crawl(node *CommandNode, currentDepth, maxDepth int, forceColor bool) error {
	if currentDepth >= maxDepth {
		return nil
	}

	// Run command with --help
	args := strings.Fields(node.FullName)
	if len(args) == 0 {
		return fmt.Errorf("empty command name")
	}

	binary := args[0]
	cmdArgs := append(args[1:], "--help")

	// Set environment to force standard width to avoid wrapping issues in docs
	// COLUMNS=80 is standard for documentation
	cmd := exec.Command(binary, cmdArgs...)
	env := append(os.Environ(), "COLUMNS=80")
	if forceColor {
		// Force color output for tools that check TTY
		env = append(env, "CLICOLOR_FORCE=1", "FORCE_COLOR=1")
	}
	cmd.Env = env

	output, err := cmd.CombinedOutput()
	if err != nil {
		c.logger.Debugf("Command '%s --help' returned error (common for some tools): %v", node.FullName, err)
		// Continue even if error, as some tools exit 1 on help
	}

	// Store both raw and cleaned output
	node.RawOutput = string(output)
	node.HelpOutput = stripANSI(node.RawOutput)

	// Find subcommands (always use cleaned output for parsing)
	subCmdNames := parseSubCommands(node.HelpOutput)

	for _, name := range subCmdNames {
		// Avoid infinite loops or standard utility subcommands
		if name == "help" || name == "completion" {
			continue
		}

		subNode := &CommandNode{
			Name:     name,
			FullName: fmt.Sprintf("%s %s", node.FullName, name),
		}

		c.logger.Debugf("Found subcommand: %s", subNode.FullName)
		node.SubCommands = append(node.SubCommands, subNode)

		// Recurse
		if err := c.crawl(subNode, currentDepth+1, maxDepth, forceColor); err != nil {
			return err
		}
	}

	return nil
}

// sortSubcommands recursively sorts subcommands based on priority order.
// Commands in the priority list appear first (in order), remaining commands are alphabetical.
func (c *Capturer) sortSubcommands(node *CommandNode, priorityOrder []string) {
	if len(node.SubCommands) == 0 {
		return
	}

	// Build priority map for O(1) lookup
	priority := make(map[string]int)
	for i, name := range priorityOrder {
		priority[name] = i
	}

	// Sort: priority commands first (by priority index), then alphabetical
	sort.SliceStable(node.SubCommands, func(i, j int) bool {
		nameI := node.SubCommands[i].Name
		nameJ := node.SubCommands[j].Name

		priI, hasI := priority[nameI]
		priJ, hasJ := priority[nameJ]

		if hasI && hasJ {
			return priI < priJ // Both have priority, sort by priority
		}
		if hasI {
			return true // i has priority, j doesn't
		}
		if hasJ {
			return false // j has priority, i doesn't
		}
		return nameI < nameJ // Neither has priority, alphabetical
	})

	// Recurse into children
	for _, child := range node.SubCommands {
		c.sortSubcommands(child, priorityOrder)
	}
}

// stripANSI removes ANSI escape codes from a string.
func stripANSI(str string) string {
	const ansi = "[\u001B\u009B][[\\]()#;?]*(?:(?:(?:[a-zA-Z\\d]*(?:;[a-zA-Z\\d]*)*)?\u0007)|(?:(?:\\d{1,4}(?:;\\d{0,4})*)?[\\dA-PRZcf-ntqry=><~]))"
	re := regexp.MustCompile(ansi)
	return re.ReplaceAllString(str, "")
}

// parseSubCommands extracts subcommand names from help text.
// It looks for a "COMMANDS" section and parses the lines following it.
func parseSubCommands(helpText string) []string {
	lines := strings.Split(helpText, "\n")
	var subcommands []string
	inCommands := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		upper := strings.ToUpper(trimmed)

		// Detect start of COMMANDS section
		// Must be a section header, not just any line containing "commands"
		// Grove tools use "COMMANDS" by itself (styled)
		// Standard cobra uses "Available Commands:"
		isCommandsHeader := trimmed == "COMMANDS" ||
			upper == "COMMANDS" ||
			upper == "AVAILABLE COMMANDS:" ||
			strings.HasPrefix(upper, "AVAILABLE COMMANDS")
		if isCommandsHeader {
			inCommands = true
			continue
		}

		if inCommands {
			// Stop at next section
			// Heuristic: All caps heading or "Flags:" or "FLAGS"
			if strings.Contains(trimmed, "FLAGS") || strings.Contains(upper, "FLAGS:") {
				break
			}
			// Check for other section headers (single word, all caps, length > 2)
			if len(trimmed) > 2 && strings.ToUpper(trimmed) == trimmed && !strings.Contains(trimmed, " ") {
				break
			}

			// Skip usage hints like "Use ... for more information"
			if strings.HasPrefix(trimmed, "Use \"") {
				continue
			}

			// Skip empty lines
			if trimmed == "" {
				continue
			}

			// Parse command name (first word)
			fields := strings.Fields(trimmed)
			if len(fields) > 0 {
				cmdName := fields[0]
				// Filter out noise/descriptions
				// Commands should be lowercase alphanumeric usually
				if !strings.ContainsAny(cmdName, ":-.") && len(cmdName) > 1 {
					subcommands = append(subcommands, cmdName)
				}
			}
		}
	}
	return subcommands
}

func (c *Capturer) render(node *CommandNode) string {
	var buf bytes.Buffer

	// Title
	buf.WriteString("# Command Reference\n\n")
	buf.WriteString(fmt.Sprintf("Reference documentation for `%s` CLI.\n\n", node.Name))

	c.renderNode(&buf, node, 2) // Start at H2

	return buf.String()
}

func (c *Capturer) renderNode(buf *bytes.Buffer, node *CommandNode, level int) {
	// Markdown Header
	prefix := strings.Repeat("#", level)
	buf.WriteString(fmt.Sprintf("%s %s\n\n", prefix, node.FullName))

	// Help Output Block
	buf.WriteString("```text\n")
	buf.WriteString(strings.TrimSpace(node.HelpOutput))
	buf.WriteString("\n```\n\n")

	// Render Children
	for _, child := range node.SubCommands {
		// Cap hierarchy depth visually at H4 to avoid deep nesting issues
		nextLevel := level + 1
		if nextLevel > 4 {
			nextLevel = 4
		}
		c.renderNode(buf, child, nextLevel)
	}
}

// renderHTML generates markdown with embedded HTML terminal blocks.
func (c *Capturer) renderHTML(node *CommandNode) string {
	var buf bytes.Buffer

	// Title
	buf.WriteString("# CLI Reference\n\n")
	buf.WriteString(fmt.Sprintf("Complete command reference for `%s`.\n\n", node.Name))

	c.renderHTMLNode(&buf, node, 2) // Start at H2

	return buf.String()
}

func (c *Capturer) renderHTMLNode(buf *bytes.Buffer, node *CommandNode, level int) {
	// Markdown header
	prefix := strings.Repeat("#", level)
	buf.WriteString(fmt.Sprintf("%s %s\n\n", prefix, node.FullName))

	// Terminal output as embedded HTML
	buf.WriteString("<div class=\"terminal\">\n")
	buf.WriteString(ansiToHTML(strings.TrimSpace(node.RawOutput)))
	buf.WriteString("\n</div>\n\n")

	// Render Children
	for _, child := range node.SubCommands {
		nextLevel := level + 1
		if nextLevel > 4 {
			nextLevel = 4
		}
		c.renderHTMLNode(buf, child, nextLevel)
	}
}

// escapeHTML escapes special HTML characters.
func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

// ansiToHTML converts ANSI escape codes to HTML spans with CSS classes.
func ansiToHTML(s string) string {
	var buf bytes.Buffer
	var currentStyles []string

	// Regex to match ANSI escape sequences
	ansiPattern := regexp.MustCompile(`\x1b\[([0-9;]*)m`)

	lastIndex := 0
	for _, match := range ansiPattern.FindAllStringSubmatchIndex(s, -1) {
		// Write text before this escape sequence
		if match[0] > lastIndex {
			text := s[lastIndex:match[0]]
			buf.WriteString(escapeHTML(text))
		}

		// Parse the SGR parameters
		params := s[match[2]:match[3]]
		newStyles := parseANSIParams(params)

		// Close previous span if we had styles
		if len(currentStyles) > 0 {
			buf.WriteString("</span>")
		}

		// Open new span if we have styles
		currentStyles = newStyles
		if len(currentStyles) > 0 {
			buf.WriteString("<span class=\"")
			buf.WriteString(strings.Join(currentStyles, " "))
			buf.WriteString("\">")
		}

		lastIndex = match[1]
	}

	// Write remaining text
	if lastIndex < len(s) {
		buf.WriteString(escapeHTML(s[lastIndex:]))
	}

	// Close any open span
	if len(currentStyles) > 0 {
		buf.WriteString("</span>")
	}

	return buf.String()
}

// parseANSIParams converts SGR parameters to CSS class names.
func parseANSIParams(params string) []string {
	if params == "" || params == "0" {
		return nil // Reset
	}

	var classes []string
	parts := strings.Split(params, ";")

	for _, p := range parts {
		switch p {
		case "0":
			return nil // Reset
		case "1":
			classes = append(classes, "term-bold")
		case "2":
			classes = append(classes, "term-dim")
		case "3":
			classes = append(classes, "term-italic")
		case "4":
			classes = append(classes, "term-underline")
		case "30":
			classes = append(classes, "term-fg-0")
		case "31":
			classes = append(classes, "term-fg-1")
		case "32":
			classes = append(classes, "term-fg-2")
		case "33":
			classes = append(classes, "term-fg-3")
		case "34":
			classes = append(classes, "term-fg-4")
		case "35":
			classes = append(classes, "term-fg-5")
		case "36":
			classes = append(classes, "term-fg-6")
		case "37":
			classes = append(classes, "term-fg-7")
		case "90":
			classes = append(classes, "term-fg-8")
		case "91":
			classes = append(classes, "term-fg-9")
		case "92":
			classes = append(classes, "term-fg-10")
		case "93":
			classes = append(classes, "term-fg-11")
		case "94":
			classes = append(classes, "term-fg-12")
		case "95":
			classes = append(classes, "term-fg-13")
		case "96":
			classes = append(classes, "term-fg-14")
		case "97":
			classes = append(classes, "term-fg-15")
		case "40":
			classes = append(classes, "term-bg-0")
		case "41":
			classes = append(classes, "term-bg-1")
		case "42":
			classes = append(classes, "term-bg-2")
		case "43":
			classes = append(classes, "term-bg-3")
		case "44":
			classes = append(classes, "term-bg-4")
		case "45":
			classes = append(classes, "term-bg-5")
		case "46":
			classes = append(classes, "term-bg-6")
		case "47":
			classes = append(classes, "term-bg-7")
		case "100":
			classes = append(classes, "term-bg-8")
		case "101":
			classes = append(classes, "term-bg-9")
		case "102":
			classes = append(classes, "term-bg-10")
		case "103":
			classes = append(classes, "term-bg-11")
		case "104":
			classes = append(classes, "term-bg-12")
		case "105":
			classes = append(classes, "term-bg-13")
		case "106":
			classes = append(classes, "term-bg-14")
		case "107":
			classes = append(classes, "term-bg-15")
		}
	}

	return classes
}
