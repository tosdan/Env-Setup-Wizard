package dotenv_test

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/tosdan/env-setup-wizard/internal/domain"
	"github.com/tosdan/env-setup-wizard/internal/dotenv"
)

func TestParseTemplatePreservesValidSourceFormat(t *testing.T) {
	tests := []struct {
		name             string
		input            []byte
		wantText         string
		wantLineEnding   domain.LineEnding
		wantFinalNewline bool
	}{
		{
			name:           "single line without final newline",
			input:          []byte("KEY=value"),
			wantText:       "KEY=value",
			wantLineEnding: domain.LineEndingLF,
		},
		{
			name:             "LF with final newline",
			input:            []byte("FIRST=one\nSECOND=two\n"),
			wantText:         "FIRST=one\nSECOND=two\n",
			wantLineEnding:   domain.LineEndingLF,
			wantFinalNewline: true,
		},
		{
			name:           "CRLF without final newline",
			input:          []byte("FIRST=one\r\nSECOND=two"),
			wantText:       "FIRST=one\r\nSECOND=two",
			wantLineEnding: domain.LineEndingCRLF,
		},
		{
			name:             "initial UTF-8 BOM is removed",
			input:            []byte("\xef\xbb\xbfKEY=value\r\n"),
			wantText:         "KEY=value\r\n",
			wantLineEnding:   domain.LineEndingCRLF,
			wantFinalNewline: true,
		},
		{
			name:             "Unicode is preserved",
			input:            []byte("GREETING='caffè ☕'\n"),
			wantText:         "GREETING='caffè ☕'\n",
			wantLineEnding:   domain.LineEndingLF,
			wantFinalNewline: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeTemplate(t, tt.input)

			document, err := dotenv.ParseTemplate(path)
			if err != nil {
				t.Fatalf("ParseTemplate() error = %v, want nil", err)
			}
			if document.LineEnding != tt.wantLineEnding {
				t.Errorf("LineEnding = %q, want %q", document.LineEnding, tt.wantLineEnding)
			}
			if document.HasFinalNewline != tt.wantFinalNewline {
				t.Errorf("HasFinalNewline = %t, want %t", document.HasFinalNewline, tt.wantFinalNewline)
			}
			if got := reconstruct(document); got != tt.wantText {
				t.Errorf("reconstructed source = %q, want %q", got, tt.wantText)
			}
		})
	}
}

func TestParseTemplateRejectsInvalidEncodingAndLineEndings(t *testing.T) {
	tests := []struct {
		name        string
		input       []byte
		wantMessage string
	}{
		{name: "invalid UTF-8", input: []byte{0xff, 'K'}, wantMessage: "template is not valid UTF-8"},
		{name: "UTF-16 little endian BOM", input: []byte{0xff, 0xfe, 'K', 0x00}, wantMessage: "UTF-16 templates are not supported"},
		{name: "UTF-16 big endian BOM", input: []byte{0xfe, 0xff, 0x00, 'K'}, wantMessage: "UTF-16 templates are not supported"},
		{name: "mixed LF then CRLF", input: []byte("FIRST=one\nSECOND=two\r\n"), wantMessage: "mixed line endings at line 2"},
		{name: "mixed CRLF then LF", input: []byte("FIRST=one\r\nSECOND=two\n"), wantMessage: "mixed line endings at line 2"},
		{name: "isolated carriage return", input: []byte("FIRST=one\rSECOND=two"), wantMessage: "isolated carriage return at line 1"},
		{name: "trailing carriage return", input: []byte("KEY=value\r"), wantMessage: "isolated carriage return at line 1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeTemplate(t, tt.input)

			_, err := dotenv.ParseTemplate(path)
			if err == nil {
				t.Fatalf("ParseTemplate() error = nil, want it to contain %q", tt.wantMessage)
			}
			if !strings.Contains(err.Error(), tt.wantMessage) {
				t.Fatalf("ParseTemplate() error = %q, want it to contain %q", err, tt.wantMessage)
			}
			if !strings.Contains(err.Error(), strconv.Quote(path)) {
				t.Fatalf("ParseTemplate() error = %q, want template path %q", err, path)
			}
		})
	}
}

func TestParseTemplateReportsReadFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.env.example")

	_, err := dotenv.ParseTemplate(path)
	if err == nil {
		t.Fatal("ParseTemplate() error = nil, want read failure")
	}
	if !strings.Contains(err.Error(), "read template") || !strings.Contains(err.Error(), path) {
		t.Fatalf("ParseTemplate() error = %q, want contextual read failure", err)
	}
}

func reconstruct(document domain.Document) string {
	var result strings.Builder
	for index, node := range document.Nodes {
		if index > 0 {
			result.WriteString(string(document.LineEnding))
		}
		result.WriteString(node.RawLine())
	}
	if document.HasFinalNewline {
		result.WriteString(string(document.LineEnding))
	}

	return result.String()
}

func writeTemplate(t *testing.T, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".env.example")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}

	return path
}
