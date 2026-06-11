package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestPrintResultsAlwaysShowsFullURLs(t *testing.T) {
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	printResults([]SearchResult{{
		Title:   "Example",
		URL:     "https://example.com/full/path?with=query#fragment",
		Content: "snippet",
	}}, 1, 0, false, true, "example query")

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	out := buf.String()
	if !strings.Contains(out, "https://example.com/full/path?with=query#fragment") {
		t.Fatalf("expected full URL in output, got:\n%s", out)
	}
}
