package generator

// DefaultPrompts provides fallback prompts for common documentation sections
var DefaultPrompts = map[string]string{
	"introduction": `# Documentation Generation Task

You are an expert technical writer creating documentation for a software project.

## Task
Write a clear, engaging introduction that:
- Explains what the project is and its purpose
- Highlights key features and benefits
- Describes the problem it solves
- Identifies the target audience

## Output Format
Provide clean, well-formatted Markdown with:
- A clear main heading
- Organized sections with subheadings as needed
- Bullet points for feature lists
- Emphasis on important points using **bold** or *italic*

Keep the tone professional yet approachable.`,

	"core-concepts": `# Core Concepts Documentation

You are documenting the fundamental concepts of this software project.

## Task
Identify and explain the core concepts, including:
- Main components or modules
- Key abstractions or interfaces
- Important data structures or models
- Central workflows or processes

## Output Format
Create a Markdown document with:
- A section for each core concept (## Concept Name)
- Clear explanation of what it is and why it matters
- Code examples demonstrating usage
- Relationships between concepts

Focus on helping new users understand the mental model of the system.`,

	"usage-patterns": `# Usage Patterns Documentation

Document common usage patterns and examples for this project.

## Task
Provide practical examples showing:
- Basic usage scenarios
- Common workflows
- Integration patterns
- Advanced use cases

## Output Format
Structure as Markdown with:
- Descriptive headings for each pattern
- Step-by-step instructions where appropriate
- Code examples with explanations
- Command-line examples if applicable

Make it easy for users to find and apply these patterns.`,

	"best-practices": `# Best Practices Documentation

Document recommended best practices for using this project effectively.

## Task
Cover important practices including:
- Performance considerations
- Security guidelines
- Code organization
- Testing strategies
- Common pitfalls to avoid

## Output Format
Provide Markdown documentation with:
- Clear headings for each practice area
- Rationale for why each practice matters
- Do's and don'ts
- Code examples showing good vs bad approaches

Help users avoid common mistakes and write better code.`,
}

// GetPromptWithFallback returns a custom prompt if it exists, otherwise falls back to default
func GetPromptWithFallback(customPath string, sectionName string) (string, error) {
	// Try to read custom prompt first
	if customPath != "" {
		content, err := os.ReadFile(customPath)
		if err == nil {
			return string(content), nil
		}
		// If file doesn't exist but was specified, that's an error
		if !os.IsNotExist(err) {
			return "", err
		}
	}
	
	// Fall back to default prompt
	if defaultPrompt, ok := DefaultPrompts[sectionName]; ok {
		return defaultPrompt, nil
	}
	
	// No prompt found
	return "", fmt.Errorf("no prompt found for section %s", sectionName)
}