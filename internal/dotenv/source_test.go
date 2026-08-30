package dotenv_test

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/tosdan/env-setup-wizard/internal/dotenv"
)

func TestLoadTemplatePreservesValidSource(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		want  dotenv.Source
	}{
		{
			name: "empty source defaults to LF",
			want: dotenv.Source{LineEnding: dotenv.LineEndingLF},
		},
		{
			name:  "single line without final newline",
			input: []byte("KEY=value"),
			want: dotenv.Source{
				Text:       "KEY=value",
				LineEnding: dotenv.LineEndingLF,
			},
		},
		{
			name:  "LF with final newline",
			input: []byte("FIRST=one\nSECOND=two\n"),
			want: dotenv.Source{
				Text:            "FIRST=one\nSECOND=two\n",
				LineEnding:      dotenv.LineEndingLF,
				HasFinalNewline: true,
			},
		},
		{
			name:  "CRLF without final newline",
			input: []byte("FIRST=one\r\nSECOND=two"),
			want: dotenv.Source{
				Text:       "FIRST=one\r\nSECOND=two",
				LineEnding: dotenv.LineEndingCRLF,
			},
		},
		{
			name:  "initial UTF-8 BOM is removed",
			input: []byte("\xef\xbb\xbfKEY=value\r\n"),
			want: dotenv.Source{
				Text:            "KEY=value\r\n",
				LineEnding:      dotenv.LineEndingCRLF,
				HasFinalNewline: true,
			},
		},
		{
			name:  "Unicode is preserved",
			input: []byte("GREETING=caffè ☕\n"),
			want: dotenv.Source{
				Text:            "GREETING=caffè ☕\n",
				LineEnding:      dotenv.LineEndingLF,
				HasFinalNewline: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeSourceFile(t, tt.input)

			got, err := dotenv.LoadTemplate(path)
			if err != nil {
				t.Fatalf("LoadTemplate() error = %v, want nil", err)
			}
			if got != tt.want {
				t.Fatalf("LoadTemplate() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestLoadTemplateRejectsInvalidEncodingAndLineEndings(t *testing.T) {
	tests := []struct {
		name        string
		input       []byte
		wantMessage string
	}{
		{
			name:        "invalid UTF-8",
			input:       []byte{0xff, 'K', 'E', 'Y'},
			wantMessage: "template is not valid UTF-8",
		},
		{
			name:        "UTF-16 little endian BOM",
			input:       []byte{0xff, 0xfe, 'K', 0x00},
			wantMessage: "UTF-16 templates are not supported",
		},
		{
			name:        "UTF-16 big endian BOM",
			input:       []byte{0xfe, 0xff, 0x00, 'K'},
			wantMessage: "UTF-16 templates are not supported",
		},
		{
			name:        "mixed LF then CRLF",
			input:       []byte("FIRST=one\nSECOND=two\r\n"),
			wantMessage: "mixed line endings at line 2",
		},
		{
			name:        "mixed CRLF then LF",
			input:       []byte("FIRST=one\r\nSECOND=two\n"),
			wantMessage: "mixed line endings at line 2",
		},
		{
			name:        "isolated carriage return",
			input:       []byte("FIRST=one\rSECOND=two"),
			wantMessage: "isolated carriage return at line 1",
		},
		{
			name:        "trailing carriage return",
			input:       []byte("KEY=value\r"),
			wantMessage: "isolated carriage return at line 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeSourceFile(t, tt.input)

			_, err := dotenv.LoadTemplate(path)
			if err == nil {
				t.Fatalf("LoadTemplate() error = nil, want it to contain %q", tt.wantMessage)
			}
			if !strings.Contains(err.Error(), tt.wantMessage) {
				t.Fatalf("LoadTemplate() error = %q, want it to contain %q", err, tt.wantMessage)
			}
			if !strings.Contains(err.Error(), strconv.Quote(path)) {
				t.Fatalf("LoadTemplate() error = %q, want template path %q", err, path)
			}
		})
	}
}

func TestLoadTemplateReportsReadFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.env.example")

	_, err := dotenv.LoadTemplate(path)
	if err == nil {
		t.Fatal("LoadTemplate() error = nil, want read failure")
	}
	if !strings.Contains(err.Error(), "read template") || !strings.Contains(err.Error(), path) {
		t.Fatalf("LoadTemplate() error = %q, want contextual read failure", err)
	}
}

func writeSourceFile(t *testing.T, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".env.example")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}

	return path
}
