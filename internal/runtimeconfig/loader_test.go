package runtimeconfig

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/buzyka/aitestkit/internal/connectorapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubConnector struct{}

func (s *stubConnector) Run(_ context.Context, _ connectorapi.PromptRequest, _ any) error {
	return nil
}

func TestLoadRuntimeFromWorkingDir(t *testing.T) {
	t.Run("returns error when go.mod is missing", func(t *testing.T) {
		ResetForTests()

		dir := t.TempDir()
		_, err := LoadRuntimeFromWorkingDir(dir)
		require.EqualError(t, err, "go.mod not found from working directory")
	})

	t.Run("returns error when config file is missing", func(t *testing.T) {
		ResetForTests()

		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/project\n")

		_, err := LoadRuntimeFromWorkingDir(dir)
		require.EqualError(t, err, ".aitestkit.json not found next to go.mod")
	})

	t.Run("returns error for invalid json", func(t *testing.T) {
		ResetForTests()

		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/project\n")
		writeFile(t, filepath.Join(dir, ConfigFileName), "{invalid")

		_, err := LoadRuntimeFromWorkingDir(dir)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "decode .aitestkit.json:")
	})

	t.Run("returns error for unsupported provider", func(t *testing.T) {
		ResetForTests()

		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/project\n")
		writeFile(t, filepath.Join(dir, ConfigFileName), `{"provider":"anthropic"}`)

		_, err := LoadRuntimeFromWorkingDir(dir)
		require.EqualError(t, err, `unsupported provider "anthropic"`)
	})

	t.Run("builds openai connector from file config", func(t *testing.T) {
		ResetForTests()

		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/project\n")
		writeFile(t, filepath.Join(dir, ConfigFileName), `{
			"provider": "openai",
			"openai": {
				"api_key_env": "OPENAI_API_KEY",
				"model": "gpt-5-mini",
				"base_url": "https://api.openai.com",
				"reasoning_effort": "minimal"
			}
		}`)

		expected := &stubConnector{}
		var captured OpenAIConfig
		SetTestHooks(TestHooks{
			BuildOpenAIConnector: func(cfg OpenAIConfig) (connectorapi.Connector, error) {
				captured = cfg
				return expected, nil
			},
		})

		runtime, err := LoadRuntimeFromWorkingDir(dir)
		require.NoError(t, err)
		assert.Same(t, expected, runtime.Connector)
		assert.Equal(t, DefaultRequestTimeout, runtime.Timeout)
		assert.Equal(t, OpenAIConfig{
			APIKeyEnv:       "OPENAI_API_KEY",
			Model:           "gpt-5-mini",
			BaseURL:         "https://api.openai.com",
			ReasoningEffort: "minimal",
		}, captured)
	})

	t.Run("builds runtime from inline api key", func(t *testing.T) {
		ResetForTests()

		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/project\n")
		writeFile(t, filepath.Join(dir, ConfigFileName), `{
			"provider": "openai",
			"timeout": "45s",
			"openai": {
				"api_key": "sk-inline-key",
				"model": "gpt-5-mini"
			}
		}`)

		runtime, err := LoadRuntimeFromWorkingDir(dir)
		require.NoError(t, err)
		require.NotNil(t, runtime.Connector)
		assert.Equal(t, 45*time.Second, runtime.Timeout)
	})
}

func TestResolveAPIKey(t *testing.T) {
	t.Run("rejects missing key source", func(t *testing.T) {
		ResetForTests()

		_, err := resolveAPIKey(OpenAIConfig{})
		require.EqualError(t, err, "exactly one of api_key or api_key_env must be set")
	})

	t.Run("rejects both key source options", func(t *testing.T) {
		ResetForTests()

		_, err := resolveAPIKey(OpenAIConfig{
			APIKey:    "x",
			APIKeyEnv: "OPENAI_API_KEY",
		})
		require.EqualError(t, err, "exactly one of api_key or api_key_env must be set")
	})

	t.Run("returns inline key", func(t *testing.T) {
		ResetForTests()

		key, err := resolveAPIKey(OpenAIConfig{APIKey: " secret "})
		require.NoError(t, err)
		assert.Equal(t, "secret", key)
	})

	t.Run("reads key from env name", func(t *testing.T) {
		ResetForTests()

		SetTestHooks(TestHooks{
			LookupEnv: func(key string) (string, bool) {
				assert.Equal(t, "OPENAI_API_KEY", key)
				return " secret ", true
			},
		})

		key, err := resolveAPIKey(OpenAIConfig{APIKeyEnv: "OPENAI_API_KEY"})
		require.NoError(t, err)
		assert.Equal(t, "secret", key)
	})

	t.Run("rejects empty env value", func(t *testing.T) {
		ResetForTests()

		SetTestHooks(TestHooks{
			LookupEnv: func(_ string) (string, bool) {
				return "", true
			},
		})

		_, err := resolveAPIKey(OpenAIConfig{APIKeyEnv: "OPENAI_API_KEY"})
		require.EqualError(t, err, `environment variable "OPENAI_API_KEY" must not be empty`)
	})
}

func TestDefaultRuntimeCachesResult(t *testing.T) {
	ResetForTests()

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/project\n")
	writeFile(t, filepath.Join(dir, ConfigFileName), `{"provider":"openai","openai":{"api_key":"secret"}}`)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "internal", "tests"), 0o755))

	buildCalls := 0
	SetTestHooks(TestHooks{
		GetWorkingDir: func() (string, error) {
			return filepath.Join(dir, "internal", "tests"), nil
		},
		BuildOpenAIConnector: func(cfg OpenAIConfig) (connectorapi.Connector, error) {
			buildCalls++
			assert.Equal(t, "secret", cfg.APIKey)
			return &stubConnector{}, nil
		},
	})

	first, err := DefaultRuntime()
	require.NoError(t, err)
	second, err := DefaultRuntime()
	require.NoError(t, err)

	assert.NotNil(t, first.Connector)
	assert.NotNil(t, second.Connector)
	assert.Equal(t, 1, buildCalls)
}

func TestDefaultRuntimeReturnsWorkingDirectoryError(t *testing.T) {
	ResetForTests()
	SetTestHooks(TestHooks{
		GetWorkingDir: func() (string, error) {
			return "", errors.New("cwd unavailable")
		},
	})

	_, err := DefaultRuntime()
	require.EqualError(t, err, "get working directory: cwd unavailable")
}

func TestResolveTimeout(t *testing.T) {
	t.Run("uses default timeout when empty", func(t *testing.T) {
		timeout, err := resolveTimeout("")
		require.NoError(t, err)
		assert.Equal(t, DefaultRequestTimeout, timeout)
	})

	t.Run("parses configured timeout", func(t *testing.T) {
		timeout, err := resolveTimeout("30s")
		require.NoError(t, err)
		assert.Equal(t, 30*time.Second, timeout)
	})

	t.Run("rejects invalid timeout", func(t *testing.T) {
		_, err := resolveTimeout("abc")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "parse timeout:")
	})

	t.Run("rejects non positive timeout", func(t *testing.T) {
		_, err := resolveTimeout("0s")
		require.EqualError(t, err, "timeout must be greater than zero")
	})
}

func TestSetDefaultRuntimeForTests(t *testing.T) {
	ResetForTests()

	expected := &stubConnector{}
	SetDefaultRuntimeForTests(Runtime{
		Connector: expected,
		Timeout:   time.Second,
	}, nil)

	runtime, err := DefaultRuntime()
	require.NoError(t, err)
	assert.Same(t, expected, runtime.Connector)
	assert.Equal(t, time.Second, runtime.Timeout)
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()

	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}
