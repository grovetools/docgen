# Configuration Documentation

You are documenting configuration for grove-docgen, covering essential setup and customization.

## Task
Create a practical configuration guide that helps users set up and customize grove-docgen for their projects.

## Topics to Document

1. **Root-Level Fields**
   - `enabled` (boolean): Enable/disable documentation generation
   - `title` (string): Package title displayed in docs
   - `description` (string): Package description
   - `category` (string): Categorization for aggregation

2. **Settings Section**
   - `model`: Available models (gemini-2.5-pro, gpt-4o, claude-3-5-sonnet, etc.)
   - `regeneration_mode`: "scratch" vs "reference" modes
   - `system_prompt`: Default or custom system prompts
   - `rules_file`: Path to .grove/rules file for context
   - `structured_output_file`: JSON output configuration

3. **Generation Parameters**
   - `max_output_tokens`: Token limits (default: 6000)
   - `temperature`: Randomness control (0.0-1.0)
   - `top_p`: Nucleus sampling parameter
   - `top_k`: Top-k sampling parameter
   - Global defaults vs per-section overrides

4. **Sections Array**
   - `name`: Internal identifier
   - `title`: Display title
   - `order`: Section ordering (1-based)
   - `prompt`: Path to prompt file
   - `output`: Generated output filename
   - `json_key`: Key for structured output
   - Model/parameter overrides per section

5. **Validation Rules**
   - Required vs optional fields
   - Value constraints and defaults
   - Schema validation via JSON schema
   - Per-section generation parameters

5. **Advanced Topics**
   - Using custom system prompts
   - Rules file format and usage
   - Integration with cx for context building
   - Working with the JSON schema

## Examples to Include
- Minimal configuration
- Full-featured configuration
- Different configurations for different project types
- Real examples from grove-tend and other projects

## Output Format
Create comprehensive Markdown documentation with:
- Clear explanations of each option
- YAML configuration examples
- Use cases and best practices
- Tips for optimization