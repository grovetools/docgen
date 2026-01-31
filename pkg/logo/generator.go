package logo

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/tdewolff/canvas"
	"github.com/tdewolff/canvas/renderers/svg"
)

// Generator handles the creation of combined logo+text SVGs.
type Generator struct {
	logger *logrus.Logger
}

// New creates a new Generator instance.
func New(logger *logrus.Logger) *Generator {
	return &Generator{logger: logger}
}

// Config holds the configuration for logo generation.
type Config struct {
	InputPath  string  // Path to the input logo SVG
	OutputPath string  // Path for the output combined SVG
	Text       string  // Text to display (e.g., "grove flow")
	TextColor  string  // Color for the text (e.g., "#589ac7")
	FontPath   string  // Path to font file (TTF/OTF) - required for path conversion
	FontSize   float64 // Font size in pixels (defaults to 48)
	Spacing    float64 // Spacing between logo and text (defaults to 20)
	TextScale  float64 // Text width as proportion of logo width (defaults to 0.8)
	Width      float64 // Output SVG width in pixels (defaults to 200)
}

// DefaultConfig returns a Config with sensible defaults.
// Note: TextColor defaults to empty string to trigger auto-detection from source SVG.
func DefaultConfig() Config {
	return Config{
		FontSize:  48,
		Spacing:   20,
		TextColor: "", // Empty = auto-detect from source SVG
		TextScale: 0.8,
		Width:     200,
	}
}

// SVGDimensions holds the parsed dimensions of an SVG.
type SVGDimensions struct {
	Width   float64
	Height  float64
	ViewBox string
	Content string // The inner content of the SVG (everything inside <svg>...</svg>)
}

// Generate creates a combined logo+text SVG with text converted to paths.
func (g *Generator) Generate(cfg Config) error {
	// Validate font path
	if cfg.FontPath == "" {
		return fmt.Errorf("font path is required for text-to-path conversion")
	}

	// Read and parse the input SVG
	dims, err := g.parseSVG(cfg.InputPath)
	if err != nil {
		return fmt.Errorf("failed to parse input SVG: %w", err)
	}

	// Apply defaults
	if cfg.FontSize == 0 {
		cfg.FontSize = DefaultConfig().FontSize
	}
	if cfg.Spacing == 0 {
		cfg.Spacing = DefaultConfig().Spacing
	}
	if cfg.TextScale == 0 {
		cfg.TextScale = DefaultConfig().TextScale
	}
	if cfg.Width == 0 {
		cfg.Width = DefaultConfig().Width
	}

	// Auto-detect text color from source SVG if not specified
	if cfg.TextColor == "" {
		cfg.TextColor = extractDominantColor(dims.Content)
		if cfg.TextColor == "" {
			cfg.TextColor = "#589ac7" // Fallback to grove blue
		}
		g.logger.Debugf("Auto-detected text color: %s", cfg.TextColor)
	}

	// Load the font
	fontFamily := canvas.NewFontFamily("custom")
	if err := fontFamily.LoadFontFile(cfg.FontPath, canvas.FontRegular); err != nil {
		return fmt.Errorf("failed to load font %s: %w", cfg.FontPath, err)
	}

	// Create a face for measuring and rendering
	face := fontFamily.Face(cfg.FontSize, canvas.Black, canvas.FontRegular, canvas.FontNormal)

	// Measure the text
	textPath, _, err := face.ToPath(cfg.Text)
	if err != nil {
		return fmt.Errorf("failed to convert text to path: %w", err)
	}
	textBounds := textPath.Bounds()
	textWidth := textBounds.W()
	textHeight := textBounds.H()

	// Parse the original viewBox
	var vbX, vbY, vbW, vbH float64
	if dims.ViewBox != "" {
		parts := strings.Fields(dims.ViewBox)
		if len(parts) == 4 {
			vbX, _ = strconv.ParseFloat(parts[0], 64)
			vbY, _ = strconv.ParseFloat(parts[1], 64)
			vbW, _ = strconv.ParseFloat(parts[2], 64)
			vbH, _ = strconv.ParseFloat(parts[3], 64)
		}
	} else {
		vbW = dims.Width
		vbH = dims.Height
	}

	// Calculate scale factor from viewBox to actual dimensions
	scaleY := dims.Height / vbH

	// Scale text to fit nicely under the logo
	targetTextWidth := vbW * cfg.TextScale
	textScale := targetTextWidth / textWidth
	scaledTextWidth := textWidth * textScale
	scaledTextHeightFinal := textHeight * textScale

	// Calculate new viewBox dimensions
	// Width: use the larger of logo width or text width, plus small margin for text
	newVBWidth := vbW
	logoOffsetX := 0.0
	textX := 0.0
	textMargin := scaledTextHeightFinal * 0.2 // Horizontal margin to prevent clipping
	if scaledTextWidth+textMargin > vbW {
		// Text is wider than logo - expand viewBox and center logo
		newVBWidth = scaledTextWidth + textMargin
		logoOffsetX = (newVBWidth - vbW) / 2
		textX = textMargin / 2
	} else {
		// Logo is wider - center text under logo
		textX = (vbW - scaledTextWidth) / 2
	}

	// Calculate new dimensions - add space for text below logo
	scaledSpacing := cfg.Spacing / scaleY
	topPadding := 20.0    // Padding at top to prevent clipping
	bottomPadding := 50.0 // Extra padding between logo and text
	newVBHeight := topPadding + vbH + scaledSpacing + bottomPadding + scaledTextHeightFinal + 10 // extra padding

	// Use specified width and calculate height to maintain aspect ratio
	newWidth := cfg.Width
	aspectRatio := newVBHeight / newVBWidth
	newHeight := newWidth * aspectRatio

	// Generate text path SVG using canvas
	textPathSVG, err := g.generateTextPathSVG(cfg.Text, face, cfg.TextColor, textScale)
	if err != nil {
		return fmt.Errorf("failed to generate text path: %w", err)
	}

	// Calculate text Y position (below logo, accounting for padding)
	textY := topPadding + vbH + scaledSpacing + bottomPadding

	// Coordinate normalization: always use (0, 0) as the output viewBox origin.
	// We translate the logo content to compensate for any non-zero origin in the source SVG.
	// This handles SVGs with centered coordinates like viewBox="-50 -50 100 100".
	// Also add topPadding to push content down from the top edge.
	normX := -vbX // Translation to normalize source coordinates to (0,0)
	normY := -vbY + topPadding

	// Generate the combined SVG with normalized coordinates (origin at 0,0)
	var buf bytes.Buffer
	buf.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	buf.WriteString(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%.0f" height="%.0f" viewBox="0 0 %.2f %.2f">`,
		newWidth, newHeight, newVBWidth, newVBHeight))
	buf.WriteString("\n")

	// Add the original SVG content as a group, translated to normalize coordinates
	// and optionally offset horizontally if text is wider than logo
	logoTranslateX := normX + logoOffsetX
	logoTranslateY := normY
	if logoTranslateX != 0 || logoTranslateY != 0 {
		buf.WriteString(fmt.Sprintf(`  <g transform="translate(%.2f, %.2f)">`, logoTranslateX, logoTranslateY))
	} else {
		buf.WriteString("  <g>")
	}
	buf.WriteString("\n")
	buf.WriteString(dims.Content)
	buf.WriteString("\n  </g>\n")

	// Add the text path, translated to position (relative to normalized coordinates)
	buf.WriteString(fmt.Sprintf(`  <g transform="translate(%.2f, %.2f) scale(%.4f)">`, textX, textY, textScale))
	buf.WriteString("\n")
	buf.WriteString(textPathSVG)
	buf.WriteString("\n  </g>\n")

	buf.WriteString("</svg>\n")

	// Ensure output directory exists
	if err := os.MkdirAll(filepath.Dir(cfg.OutputPath), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Write the output file
	if err := os.WriteFile(cfg.OutputPath, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write output SVG: %w", err)
	}

	g.logger.Debugf("Generated combined SVG with text paths: %s", cfg.OutputPath)
	return nil
}

// generateTextPathSVG converts text to SVG path elements.
func (g *Generator) generateTextPathSVG(text string, face *canvas.FontFace, hexColor string, scale float64) (string, error) {
	// Convert text to path
	textPath, _, err := face.ToPath(text)
	if err != nil {
		return "", err
	}

	// Create a canvas context to render to SVG
	bounds := textPath.Bounds()

	// Create a small canvas just for the text
	c := canvas.New(bounds.W(), bounds.H())
	ctx := canvas.NewContext(c)

	// Parse the color
	fillColor := canvas.Black
	if hexColor != "" {
		fillColor = canvas.Hex(hexColor)
	}

	// Draw the text path
	ctx.SetFillColor(fillColor)
	ctx.DrawPath(0, bounds.H(), textPath)

	// Render to SVG
	var buf bytes.Buffer
	svgRenderer := svg.New(&buf, c.W, c.H, nil)
	c.RenderTo(svgRenderer)
	svgRenderer.Close()

	// Extract just the path elements from the SVG output
	svgContent := buf.String()
	pathContent := extractPathElements(svgContent)

	return pathContent, nil
}

// extractPathElements extracts path elements from an SVG string.
func extractPathElements(svgContent string) string {
	// Find all <path> elements
	pathRe := regexp.MustCompile(`(?s)<path[^>]*/>|<path[^>]*>.*?</path>`)
	matches := pathRe.FindAllString(svgContent, -1)
	return strings.Join(matches, "\n    ")
}

// parseSVG reads an SVG file and extracts its dimensions and inner content.
func (g *Generator) parseSVG(path string) (*SVGDimensions, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	content := string(data)
	dims := &SVGDimensions{}

	// Extract width and height attributes
	widthRe := regexp.MustCompile(`\bwidth="([^"]+)"`)
	heightRe := regexp.MustCompile(`\bheight="([^"]+)"`)
	viewBoxRe := regexp.MustCompile(`\bviewBox="([^"]+)"`)

	if m := widthRe.FindStringSubmatch(content); len(m) > 1 {
		dims.Width, _ = strconv.ParseFloat(strings.TrimSuffix(m[1], "px"), 64)
	}
	if m := heightRe.FindStringSubmatch(content); len(m) > 1 {
		dims.Height, _ = strconv.ParseFloat(strings.TrimSuffix(m[1], "px"), 64)
	}
	if m := viewBoxRe.FindStringSubmatch(content); len(m) > 1 {
		dims.ViewBox = m[1]
	}

	// Extract the inner content (everything between <svg ...> and </svg>)
	svgStartRe := regexp.MustCompile(`(?s)<svg[^>]*>`)
	svgEndRe := regexp.MustCompile(`(?s)</svg>`)

	startMatch := svgStartRe.FindStringIndex(content)
	endMatch := svgEndRe.FindStringIndex(content)

	if startMatch != nil && endMatch != nil && startMatch[1] < endMatch[0] {
		dims.Content = strings.TrimSpace(content[startMatch[1]:endMatch[0]])
	}

	// Strip Inkscape/Sodipodi metadata elements (they use namespaces not declared in our output)
	dims.Content = stripInkscapeMetadata(dims.Content)

	// Default dimensions if not found
	if dims.Width == 0 {
		dims.Width = 200
	}
	if dims.Height == 0 {
		dims.Height = 200
	}

	return dims, nil
}

// stripInkscapeMetadata removes Inkscape and Sodipodi specific elements that aren't needed for rendering.
func stripInkscapeMetadata(content string) string {
	// Remove sodipodi:namedview elements
	namedViewRe := regexp.MustCompile(`(?s)<sodipodi:namedview[^>]*/>|<sodipodi:namedview[^>]*>.*?</sodipodi:namedview>`)
	content = namedViewRe.ReplaceAllString(content, "")

	// Remove empty defs elements
	emptyDefsRe := regexp.MustCompile(`(?s)<defs[^>]*/>\s*|<defs[^>]*>\s*</defs>`)
	content = emptyDefsRe.ReplaceAllString(content, "")

	// Remove metadata elements
	metadataRe := regexp.MustCompile(`(?s)<metadata[^>]*>.*?</metadata>`)
	content = metadataRe.ReplaceAllString(content, "")

	// Remove inkscape: and sodipodi: attributes from remaining elements
	inkscapeAttrRe := regexp.MustCompile(`\s+(inkscape|sodipodi):[a-zA-Z-]+="[^"]*"`)
	content = inkscapeAttrRe.ReplaceAllString(content, "")

	return content
}

// xmlEscape escapes special XML characters in a string.
func xmlEscape(s string) string {
	var buf bytes.Buffer
	if err := xml.EscapeText(&buf, []byte(s)); err != nil {
		return s
	}
	return buf.String()
}

// extractDominantColor extracts the first non-black, non-white hex color from SVG content.
// It looks for colors in fill attributes, style attributes, and stop-color (for gradients).
// Returns empty string if no suitable color is found.
func extractDominantColor(svgContent string) string {
	// Match hex colors in various formats: fill="#abc123", fill:#abc123, stop-color="#abc123"
	// Support both 6-digit and 3-digit hex colors
	hexColorRe := regexp.MustCompile(`(?:fill|stop-color|stroke)\s*[:=]\s*["']?(#[0-9a-fA-F]{6}|#[0-9a-fA-F]{3})["']?`)

	matches := hexColorRe.FindAllStringSubmatch(svgContent, -1)

	// Skip black and white colors (we lowercase before checking, so only need lowercase entries)
	skipColors := map[string]bool{
		"#000000": true,
		"#ffffff": true,
	}

	for _, match := range matches {
		if len(match) > 1 {
			color := strings.ToLower(match[1])
			// Expand 3-digit hex to 6-digit for comparison
			if len(color) == 4 {
				color = "#" + string(color[1]) + string(color[1]) +
					string(color[2]) + string(color[2]) +
					string(color[3]) + string(color[3])
			}
			if !skipColors[color] {
				return color
			}
		}
	}

	return ""
}
