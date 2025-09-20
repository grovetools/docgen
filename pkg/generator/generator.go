package generator

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mattsolo1/grove-docgen/pkg/config"
	"github.com/sirupsen/logrus"
)

// Generator handles the documentation generation for a single package.
type Generator struct {
	logger *logrus.Logger
}

func New(logger *logrus.Logger) *Generator {
	return &Generator{logger: logger}
}

// Generate reads a config, builds context, calls an LLM, and writes the output.
func (g *Generator) Generate(packageDir string) error {
	g.logger.Infof("Generating documentation for package at: %s", packageDir)

	// 1. Load config
	cfg, err := config.Load(packageDir)
	if err != nil {
		return fmt.Errorf("failed to load docgen config: %w", err)
	}

	// 2. Build context using `cx`
	g.logger.Info("Building context with 'cx generate'...")
	if err := g.buildContext(packageDir); err != nil {
		return fmt.Errorf("failed to build context: %w", err)
	}

	// 3. Generate each section
	for _, section := range cfg.Sections {
		g.logger.Infof("Generating section: %s", section.Name)
		
		promptPath := filepath.Join(packageDir, "docs", section.Prompt)
		output, err := g.callLLM(promptPath, cfg.Model)
		if err != nil {
			g.logger.WithError(err).Errorf("LLM call failed for section '%s'", section.Name)
			continue // Continue to the next section even if one fails
		}

		// 4. Write output
		outputPath := filepath.Join(packageDir, "docs", "dist", section.Output)
		if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}
		if err := os.WriteFile(outputPath, []byte(output), 0644); err != nil {
			return fmt.Errorf("failed to write section output: %w", err)
		}
		g.logger.Infof("Successfully wrote section '%s' to %s", section.Name, outputPath)
	}

	return nil
}

func (g *Generator) buildContext(packageDir string) error {
	// cx generate uses the .grove/rules file if it exists, relative to the CWD.
	cmd := exec.Command("cx", "generate")
	cmd.Dir = packageDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (g *Generator) callLLM(promptPath string, model string) (string, error) {
	// Use provided model or default to gemini-1.5-flash-latest
	if model == "" {
		model = "gemini-1.5-flash-latest"
	}
	
	// This reuses the simple gemapi call from grove-tend's generator.
	// We assume gemapi is configured and available in the PATH.
	args := []string{
		"request",
		"--model", model,
		"--file", promptPath,
		"--yes",
	}
	cmd := exec.Command("gemapi", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("gemapi request failed: %w\nOutput: %s", err, string(output))
	}

	// Clean up potential markdown code fences from the output
	response := string(output)
	response = strings.TrimSpace(response)
	if strings.HasPrefix(response, "```") && strings.HasSuffix(response, "```") {
		lines := strings.Split(response, "\n")
		response = strings.Join(lines[1:len(lines)-1], "\n")
	}

	return response, nil
}