package recipes

import "embed"

//go:embed all:builtin/docgen-customize-agent
var DocgenCustomizeAgentFS embed.FS

//go:embed all:builtin/docgen-customize-prompts
var DocgenCustomizePromptsFS embed.FS

//go:embed all:builtin/add-readme-template
var AddReadmeTemplateFS embed.FS