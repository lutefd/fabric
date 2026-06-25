package cli

import (
	"errors"
	"os"
	"testing"
)

func TestRenderedAndTypedErrors(t *testing.T) {
	cause := errors.New("boom")
	rendered := renderedError{cause: cause}
	if rendered.Error() != "boom" || !errors.Is(rendered, cause) || !IsRenderedError(rendered) {
		t.Fatalf("rendered error not wrapping cause: %v", rendered)
	}

	typed := typedError("custom_code", "custom message", map[string]any{"id": "1"})
	if typed.Error() != "custom message" || errorCode(typed) != "custom_code" || errorDetails(typed)["id"] != "1" {
		t.Fatalf("typed error helpers failed: %v", typed)
	}
}

func TestExtractOutputFormatVariantsAndErrors(t *testing.T) {
	format, clean, err := extractOutputFormat([]string{"--format=json", "version"})
	if err != nil || format != "json" || len(clean) != 1 || clean[0] != "version" {
		t.Fatalf("format=%q clean=%v err=%v", format, clean, err)
	}

	format, clean, err = extractOutputFormat([]string{"--format", "human", "version"})
	if err != nil || format != "human" || len(clean) != 1 || clean[0] != "version" {
		t.Fatalf("format=%q clean=%v err=%v", format, clean, err)
	}

	for _, args := range [][]string{{"--format"}, {"--format=xml"}} {
		if _, _, err := extractOutputFormat(args); err == nil {
			t.Fatalf("extractOutputFormat(%v) succeeded", args)
		}
	}
}

func TestRunJSONOutputFallbackAndErrorEnvelope(t *testing.T) {
	output := captureStdout(t, func() {
		if err := Run([]string{"--json"}); err != nil {
			t.Fatal(err)
		}
	})
	assertContains(t, output, `"command":"help"`)
	assertContains(t, output, `"message":"Fabric`)

	output = captureStdout(t, func() {
		err := Run([]string{"--format=json", "version", "extra"})
		if err == nil || !IsRenderedError(err) {
			t.Fatalf("err = %v", err)
		}
	})
	assertContains(t, output, `"ok":false`)
	assertContains(t, output, `"code":"internal_error"`)
}

func TestErrorCodeClassifiesMessages(t *testing.T) {
	cases := map[string]string{
		"not found":          "not_found",
		"unknown thread":     "not_found",
		"conflict happened":  "conflict",
		"immutable mismatch": "conflict",
		"requires value":     "invalid_argument",
		"must be set":        "invalid_argument",
		"something else":     "internal_error",
	}
	for message, want := range cases {
		if got := errorCode(errors.New(message)); got != want {
			t.Fatalf("errorCode(%q) = %q, want %q", message, got, want)
		}
	}
}

func TestVersionAndCapabilitiesRejectArguments(t *testing.T) {
	if err := runVersion([]string{"extra"}); err == nil {
		t.Fatal("runVersion accepted arguments")
	}
	if err := runCapabilities([]string{"extra"}); err == nil {
		t.Fatal("runCapabilities accepted arguments")
	}
}

func TestPromptDurabilityChoicesAndReadError(t *testing.T) {
	for input, want := range map[string]string{
		"y\n":         DurabilityDurable,
		"later\n":     DurabilityCandidate,
		"something\n": DurabilityLive,
	} {
		got := withStdin(t, input, func() string {
			value, err := promptDurability()
			if err != nil {
				t.Fatal(err)
			}
			return value
		})
		if got != want {
			t.Fatalf("promptDurability(%q) = %q, want %q", input, got, want)
		}
	}

	oldStdin := os.Stdin
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	os.Stdin = reader
	defer func() {
		os.Stdin = oldStdin
	}()
	value, err := promptDurability()
	if err != nil || value != DurabilityLive {
		t.Fatalf("promptDurability read error value=%q err=%v", value, err)
	}
}

func withStdin(t *testing.T, input string, fn func() string) string {
	t.Helper()
	oldStdin := os.Stdin
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.WriteString(input); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	os.Stdin = reader
	defer func() {
		os.Stdin = oldStdin
		reader.Close()
	}()
	return fn()
}
