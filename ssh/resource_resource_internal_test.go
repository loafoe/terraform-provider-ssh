package ssh

import (
	"strings"
	"testing"

	"github.com/hashicorp/go-cty/cty"
)

func TestConfiguredFiles(t *testing.T) {
	fileObject := func(destination string, source, content cty.Value) cty.Value {
		return cty.ObjectVal(map[string]cty.Value{
			"destination": cty.StringVal(destination),
			"source":      source,
			"content":     content,
		})
	}
	rawConfigWithFiles := func(files ...cty.Value) cty.Value {
		return cty.ObjectVal(map[string]cty.Value{
			"file": cty.SetVal(files),
		})
	}
	nullString := cty.NullVal(cty.String)

	tests := []struct {
		name     string
		config   cty.Value
		expected map[string]provisionFilePresence
	}{
		{
			name:     "null config",
			config:   cty.NullVal(cty.DynamicPseudoType),
			expected: map[string]provisionFilePresence{},
		},
		{
			name: "empty content is configured",
			config: rawConfigWithFiles(
				fileObject("/tmp/empty", nullString, cty.StringVal("")),
			),
			expected: map[string]provisionFilePresence{
				"/tmp/empty": {Content: true},
			},
		},
		{
			name: "source is configured",
			config: rawConfigWithFiles(
				fileObject("/tmp/source", cty.StringVal("./file.txt"), nullString),
			),
			expected: map[string]provisionFilePresence{
				"/tmp/source": {Source: true},
			},
		},
		{
			name: "neither source nor content is configured",
			config: rawConfigWithFiles(
				fileObject("/tmp/missing", nullString, nullString),
			),
			expected: map[string]provisionFilePresence{
				"/tmp/missing": {},
			},
		},
		{
			name: "unknown content is treated as configured",
			config: rawConfigWithFiles(
				fileObject("/tmp/unknown", nullString, cty.UnknownVal(cty.String)),
			),
			expected: map[string]provisionFilePresence{
				"/tmp/unknown": {Content: true},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := configuredFiles(tt.config)
			if len(got) != len(tt.expected) {
				t.Fatalf("expected %d entries, got %d: %#v", len(tt.expected), len(got), got)
			}
			for destination, expectedPresence := range tt.expected {
				if got[destination] != expectedPresence {
					t.Fatalf("file %q: expected presence %#v, got %#v", destination, expectedPresence, got[destination])
				}
			}
		})
	}
}

func TestValidateProvisionFile(t *testing.T) {
	tests := []struct {
		name        string
		file        provisionFile
		presence    provisionFilePresence
		errorDetail string
	}{
		{
			name:     "non-empty content",
			file:     provisionFile{Destination: "/tmp/file", Content: "hello"},
			presence: provisionFilePresence{Content: true},
		},
		{
			name:     "explicitly empty content",
			file:     provisionFile{Destination: "/tmp/file"},
			presence: provisionFilePresence{Content: true},
		},
		{
			name:     "source",
			file:     provisionFile{Destination: "/tmp/file", Source: "./file.txt"},
			presence: provisionFilePresence{Source: true},
		},
		{
			name:        "neither source nor content",
			file:        provisionFile{Destination: "/tmp/file"},
			errorDetail: "has neither 'source' nor 'content' set",
		},
		{
			name:        "empty source",
			file:        provisionFile{Destination: "/tmp/file"},
			presence:    provisionFilePresence{Source: true},
			errorDetail: "has an empty 'source'",
		},
		{
			name:        "source and content",
			file:        provisionFile{Destination: "/tmp/file", Source: "./file.txt", Content: "hello"},
			presence:    provisionFilePresence{Source: true, Content: true},
			errorDetail: "has conflicting 'source' and 'content'",
		},
		{
			name:        "source with explicitly empty content",
			file:        provisionFile{Destination: "/tmp/file", Source: "./file.txt"},
			presence:    provisionFilePresence{Source: true, Content: true},
			errorDetail: "has conflicting 'source' and 'content'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validateProvisionFile(tt.file, tt.presence)
			if tt.errorDetail == "" {
				if got != nil {
					t.Fatalf("unexpected diagnostic: %#v", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("expected diagnostic containing %q, got nil", tt.errorDetail)
			}
			if !strings.Contains(got.Detail, tt.errorDetail) {
				t.Fatalf("expected diagnostic detail containing %q, got %q", tt.errorDetail, got.Detail)
			}
		})
	}
}
