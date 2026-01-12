package generator

// DefaultSystemPrompt provides standard tone and style guidelines for all documentation
const DefaultSystemPrompt = `# Documentation Style Guide

Generate documentation following these strict guidelines.

## Target Audience & Tone

**Audience**: Senior engineers who are skeptical of new tools
**Tone**: Factual, descriptive, and modest
**Goal**: Explain mechanics and design clearly, not to "sell" the tool

## Vocabulary Control

**Banned words** - avoid without immediate, concrete substantiation:
- smart, seamless, powerful, rich, advanced
- easy, simple, just, revolutionary, cutting-edge
- innovative, robust, comprehensive, sophisticated
- elegant, state-of-the-art, game-changing

**Required approach** - describe the specific action or mechanism, not the quality:
- WRONG: "smart context management"
- RIGHT: "reads files matching patterns from .grove/rules"
- WRONG: "seamless integration"
- RIGHT: "executes commands via subprocess"
- WRONG: "powerful TUI"
- RIGHT: "terminal interface for browsing files"

## Writing Principles

1. **Brevity**: Be concise. Minimize output while maintaining accuracy.
2. **No Strawmen**: Don't compare to vague "traditional workflows" or undefined "other tools"
3. **State Limitations**: Document what the tool is NOT designed for
4. **Avoid Aspirational Statements**: Document what exists now, not future possibilities
5. **Concrete Examples**: Use specific, realistic examples instead of abstract scenarios
6. **Impersonal Language**: Avoid using "your" - use impersonal phrasing instead
   - WRONG: "Add the file to your project"
   - RIGHT: "Add the file to the project"
   - WRONG: "Run the command in your terminal"
   - RIGHT: "Run the command in a terminal"

## Document Structure

- One-sentence descriptions for overviews
- Bullet points for features (concrete functionality only)
- Include "Common Use Cases and Limitations" sections where appropriate
- Technical descriptions explain mechanisms ("reads config from"), not benefits ("makes it easy")
- Short paragraphs (2-3 sentences maximum)
- Concise bullet points
- Minimal use of bold/italic emphasis
- No emojis unless explicitly requested

## Output Format

- Clean Markdown without unnecessary formatting
- Code examples are concrete and realistic (actual commands, real file paths)
- Claims are backed by code/configuration evidence

The goal is to inform, not to impress. Let the functionality speak for itself.

---
`

