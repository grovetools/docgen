package generator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/sirupsen/logrus"
)

// TestWriteUsageReportFailedSections verifies the usage report surfaces the
// run's failed sections (the machine-readable contract `grove release gen`
// uses for section-scoped retries) and that a clean run writes an explicit
// empty list rather than omitting the field.
func TestWriteUsageReportFailedSections(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(os.Stderr)

	readReport := func(t *testing.T, path string) UsageReport {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading report: %v", err)
		}
		var r UsageReport
		if err := json.Unmarshal(data, &r); err != nil {
			t.Fatalf("unmarshaling report: %v", err)
		}
		return r
	}

	t.Run("failed sections recorded", func(t *testing.T) {
		g := New(logger)
		g.failedSections = []string{"03-workflows", "overview/introduction"}
		path := filepath.Join(t.TempDir(), "usage.json")
		g.writeUsageReport(path, "claude-haiku-4-5")

		r := readReport(t, path)
		if len(r.FailedSections) != 2 || r.FailedSections[0] != "03-workflows" || r.FailedSections[1] != "overview/introduction" {
			t.Fatalf("unexpected failed_sections: %v", r.FailedSections)
		}
	})

	t.Run("clean run writes empty list", func(t *testing.T) {
		g := New(logger)
		path := filepath.Join(t.TempDir(), "usage.json")
		g.writeUsageReport(path, "claude-haiku-4-5")

		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading report: %v", err)
		}
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(data, &raw); err != nil {
			t.Fatalf("unmarshaling report: %v", err)
		}
		fs, ok := raw["failed_sections"]
		if !ok {
			t.Fatal("failed_sections field missing from a clean run's report")
		}
		if string(fs) != "[]" {
			t.Fatalf("expected empty failed_sections list, got %s", fs)
		}
	})
}
