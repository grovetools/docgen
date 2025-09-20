# CLI Reference Documentation

You are creating a comprehensive CLI reference for grove-docgen.

## Task
Document all CLI commands, their options, and usage examples.

## Commands to Document

1. **docgen** (root command)
   - Global flags (--verbose, --json, --config)
   - Version information

2. **docgen init**
   - Purpose: Scaffold new documentation configuration
   - Options:
     - --type (currently supports "library")
   - Output structure
   - Examples

3. **docgen generate**
   - Purpose: Generate documentation for current package
   - Options:
     - --section/-s (selective generation)
   - Process overview (cloning, context building, generation)
   - Examples for different scenarios

4. **docgen aggregate**
   - Purpose: Collect documentation from multiple packages
   - Options:
     - --output-dir/-o
   - Use with grove-website
   - Manifest.json structure
   - Examples

5. **docgen version**
   - Shows version information
   - Build details

## Important Details
- Include exact flag names and aliases
- Show both short and long-form examples
- Explain what each command actually does behind the scenes
- Include common troubleshooting scenarios
- Show output examples where relevant

## Output Format
Create a structured reference guide with:
- Command syntax
- Description
- Options table
- Multiple examples
- Expected output
- Common issues and solutions