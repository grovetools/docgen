# Configuration Guide Documentation

You are documenting the comprehensive configuration options for grove-docgen.

## Task
Create a detailed configuration guide covering all aspects of docgen.config.yml.

## Configuration Areas to Document

1. **Basic Configuration**
   - enabled, title, description, category fields
   - File location and naming conventions

2. **Settings Section**
   - model selection and options
   - regeneration_mode (scratch vs reference)
   - rules_file for custom context
   - structured_output_file for JSON generation
   - system_prompt configuration

3. **Generation Parameters**
   - max_output_tokens (controlling length)
   - temperature (creativity vs determinism)
   - top_p and top_k (sampling parameters)
   - Global vs per-section overrides

4. **Sections Configuration**
   - Section structure (name, title, order, prompt, output)
   - json_key for structured output
   - Per-section model overrides
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