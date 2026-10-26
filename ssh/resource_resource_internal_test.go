package ssh

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
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

func TestTriggersFromRawConfig(t *testing.T) {
	tests := []struct {
		name     string
		config   cty.Value
		expected map[string]interface{}
		ok       bool
	}{
		{
			name:   "null config",
			config: cty.NullVal(cty.DynamicPseudoType),
		},
		{
			name: "unset triggers",
			config: cty.ObjectVal(map[string]cty.Value{
				"triggers": cty.NullVal(cty.Map(cty.String)),
			}),
			expected: map[string]interface{}{},
			ok:       true,
		},
		{
			name: "configured triggers",
			config: cty.ObjectVal(map[string]cty.Value{
				"triggers": cty.MapVal(map[string]cty.Value{
					"config_md5": cty.StringVal("abc"),
					"always_run": cty.StringVal("1"),
				}),
			}),
			expected: map[string]interface{}{
				"config_md5": "abc",
				"always_run": "1",
			},
			ok: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := triggersFromRawConfig(tt.config)
			if ok != tt.ok {
				t.Fatalf("expected ok=%t, got %t", tt.ok, ok)
			}
			if len(got) != len(tt.expected) {
				t.Fatalf("expected %d triggers, got %d: %#v", len(tt.expected), len(got), got)
			}
			for key, expected := range tt.expected {
				if got[key] != expected {
					t.Fatalf("trigger %q: expected %#v, got %#v", key, expected, got[key])
				}
			}
		})
	}
}

func TestTriggerCount(t *testing.T) {
	triggerMap := cty.MapVal(map[string]cty.Value{
		"a": cty.StringVal("1"),
		"b": cty.StringVal("2"),
	})

	tests := []struct {
		name     string
		raw      cty.Value
		expected int
		ok       bool
	}{
		{
			name: "null config",
			raw:  cty.NullVal(cty.DynamicPseudoType),
		},
		{
			name: "unset triggers",
			raw: cty.ObjectVal(map[string]cty.Value{
				"triggers": cty.NullVal(cty.Map(cty.String)),
			}),
			ok: true,
		},
		{
			name: "configured triggers",
			raw: cty.ObjectVal(map[string]cty.Value{
				"triggers": triggerMap,
			}),
			expected: 2,
			ok:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := triggerCount(tt.raw)
			if ok != tt.ok {
				t.Fatalf("expected ok=%t, got %t", tt.ok, ok)
			}
			if got != tt.expected {
				t.Fatalf("expected count %d, got %d", tt.expected, got)
			}
		})
	}
}

func TestCustomDiffMarksChangedTriggersComputed(t *testing.T) {
	state := &terraform.InstanceState{
		ID: "123",
		Attributes: map[string]string{
			"host":                               "example.com",
			"user":                               "root",
			"agent":                              "true",
			"when":                               "create",
			"port":                               "22",
			"bastion_port":                       "22",
			"timeout":                            "5m",
			"retry_delay":                        "10s",
			"commands_after_file_changes":        "true",
			"ignore_no_supported_methods_remain": "false",
			"triggers.%":                         "1",
			"triggers.config_md5":                "old",
			"result":                             "old result",
		},
	}
	config := terraform.NewResourceConfigRaw(map[string]interface{}{
		"host":  "example.com",
		"user":  "root",
		"agent": true,
		"triggers": map[string]interface{}{
			"config_md5": "new",
		},
	})

	diff, err := resourceResource().Diff(context.Background(), state, config, nil)
	if err != nil {
		t.Fatalf("unexpected diff error: %v", err)
	}
	if diff == nil {
		t.Fatal("expected a diff")
	}
	if !diff.RequiresNew() {
		t.Fatal("expected trigger change to require replacement")
	}
	triggersDiff := diff.Attributes["triggers.%"]
	if triggersDiff == nil || !triggersDiff.NewComputed {
		t.Fatalf("expected triggers to be marked computed, got %#v", triggersDiff)
	}
	resultDiff := diff.Attributes["result"]
	if resultDiff == nil || !resultDiff.NewComputed {
		t.Fatalf("expected result to be marked computed, got %#v", resultDiff)
	}
}

type fakeRunResult struct {
	stdout string
	stderr string
	done   bool
	err    error
}

type fakeCommandRunner struct {
	results  []fakeRunResult
	calls    int
	timeouts []time.Duration
}

func (f *fakeCommandRunner) Run(_ string, timeout ...time.Duration) (string, string, bool, error) {
	f.calls++
	if len(timeout) > 0 {
		f.timeouts = append(f.timeouts, timeout[0])
	}
	resultIndex := f.calls - 1
	if resultIndex >= len(f.results) {
		resultIndex = len(f.results) - 1
	}
	result := f.results[resultIndex]
	return result.stdout, result.stderr, result.done, result.err
}

func TestRunCommandsSuccess(t *testing.T) {
	runner := &fakeCommandRunner{results: []fakeRunResult{{stdout: "ok", done: true}}}
	config := SSHRetryConfig{timeout: time.Minute, retryDelay: time.Millisecond}

	stdout, diags, err := runCommands(context.Background(), []string{"hostname"}, runner, config, testConfig(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %#v", diags)
	}
	if stdout != "ok" {
		t.Fatalf("expected stdout %q, got %q", "ok", stdout)
	}
	if runner.calls != 1 {
		t.Fatalf("expected one call, got %d", runner.calls)
	}
}

func TestRunCommandsCommandTimeoutFailsWithoutRetry(t *testing.T) {
	runner := &fakeCommandRunner{results: []fakeRunResult{{stdout: "partial", stderr: "still running", done: false}}}
	config := SSHRetryConfig{timeout: time.Second, retryDelay: time.Millisecond}

	_, diags, err := runCommands(context.Background(), []string{"sleep 300"}, runner, config, testConfig(t))
	if err == nil || !strings.Contains(err.Error(), "timed out after") {
		t.Fatalf("expected timeout error, got %v", err)
	}
	if runner.calls != 1 {
		t.Fatalf("expected command timeout to fail without retrying, got %d calls", runner.calls)
	}
	if len(diags) == 0 {
		t.Fatal("expected diagnostics for command timeout")
	}
}

func TestRunCommandsAuthenticationFailureFailsFast(t *testing.T) {
	authErr := errors.New("ssh: handshake failed: no supported methods remain")
	runner := &fakeCommandRunner{results: []fakeRunResult{{err: authErr}}}
	config := SSHRetryConfig{timeout: time.Minute, retryDelay: time.Millisecond}

	_, diags, err := runCommands(context.Background(), []string{"true"}, runner, config, testConfig(t))
	if !errors.Is(err, authErr) {
		t.Fatalf("expected authentication error, got %v", err)
	}
	if runner.calls != 1 {
		t.Fatalf("expected authentication failure to fail without retrying, got %d calls", runner.calls)
	}
	if len(diags) == 0 {
		t.Fatal("expected diagnostics for authentication failure")
	}
}

func TestRunCommandsRetriesAuthenticationFailureWhenConfigured(t *testing.T) {
	authErr := errors.New("ssh: handshake failed: no supported methods remain")
	runner := &fakeCommandRunner{results: []fakeRunResult{{err: authErr}}}
	config := SSHRetryConfig{
		timeout:                      time.Second,
		retryDelay:                   time.Millisecond,
		ignoreUnsupportedAuthMethods: true,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, _, err := runCommands(ctx, []string{"true"}, runner, config, testConfig(t))
	if !errors.Is(err, authErr) {
		t.Fatalf("expected authentication error, got %v", err)
	}
	if runner.calls < 2 {
		t.Fatalf("expected authentication error to be retried, got %d calls", runner.calls)
	}
}

func TestIsUnsupportedAuthError(t *testing.T) {
	if isUnsupportedAuthError(nil) {
		t.Fatal("nil error should not be an unsupported auth error")
	}
	if !isUnsupportedAuthError(errors.New("ssh: no supported methods remain")) {
		t.Fatal("expected unsupported auth error to be detected")
	}
	if isUnsupportedAuthError(errors.New("connection refused")) {
		t.Fatal("unexpected unsupported auth error detection")
	}
}

func TestBoundedCommandTimeout(t *testing.T) {
	configured := 5 * time.Minute

	got, err := boundedCommandTimeout(context.Background(), configured)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != configured {
		t.Fatalf("expected configured timeout %s, got %s", configured, got)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	got, err = boundedCommandTimeout(ctx, configured)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got <= 0 || got > 500*time.Millisecond {
		t.Fatalf("expected timeout bounded by context deadline, got %s", got)
	}

	expiredCtx, expiredCancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer expiredCancel()
	if _, err := boundedCommandTimeout(expiredCtx, configured); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context deadline error, got %v", err)
	}
}

func testConfig(t *testing.T) *Config {
	t.Helper()
	debugFile, err := os.CreateTemp(t.TempDir(), "provider-debug-*.log")
	if err != nil {
		t.Fatalf("creating debug file: %v", err)
	}
	t.Cleanup(func() {
		_ = debugFile.Close()
	})
	return &Config{debugFile: debugFile}
}
