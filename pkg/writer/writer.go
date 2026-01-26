package writer

// Writer abstracts output format for different static site generators.
// This allows docgen to support multiple SSGs like Astro, Hugo, Docusaurus, etc.
type Writer interface {
	// WriteDoc writes transformed markdown to the appropriate location
	WriteDoc(pkg, filename string, content []byte, meta DocMetadata) error

	// WriteAsset copies an asset (image, video, cast) to the appropriate location
	WriteAsset(pkg, assetType, filename string, data []byte) error

	// WriteManifest writes the manifest file
	WriteManifest(manifest []byte) error

	// TransformContent applies SSG-specific transformations (paths, frontmatter)
	TransformContent(content []byte, pkg string, meta DocMetadata) ([]byte, error)

	// WebsiteDir returns the target website directory
	WebsiteDir() string
}

// DocMetadata contains metadata about a documentation file
type DocMetadata struct {
	Title       string
	Description string
	Category    string
	Version     string
	Order       int
	Package     string // Package title (for display)
}
