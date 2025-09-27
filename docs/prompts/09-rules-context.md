# Rules & Context Documentation

You are documenting how grove-docgen integrates with Grove's context management through rules files.

## Task
Create a guide on using rules files and context management for documentation generation.

## Topics to Cover

1. **Rules File Integration**
   - Using `rules_file` configuration
   - Default rules file locations
   - Rules file syntax and patterns
   - Gitignore-style pattern matching

2. **Context Management**
   - Hot vs cold context in documentation
   - Including relevant source files
   - Excluding test and build files
   - Optimizing context size

3. **Grove Integration**
   - Using grove-context with docgen
   - Shared rules across tools
   - Workspace-aware documentation
   - Cross-package context

4. **Pattern Strategies**
   - Documentation-specific patterns
   - Including example files
   - README and docs inclusion
   - Configuration file context

5. **Performance Optimization**
   - Balancing context size and quality
   - Token budget management
   - Selective context loading
   - Caching strategies

## Practical Examples
Show rules files for:
- API-heavy libraries
- CLI applications
- Full-stack applications
- Monorepo documentation

## Troubleshooting
- Debugging context issues
- Missing file problems
- Performance bottlenecks
- Context overflow handling