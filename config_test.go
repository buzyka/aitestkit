package aitestkit

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConnectorFromWorkingDir(t *testing.T) {
	t.Run("returns error when go.mod is missing", func(t *testing.T) {
		withConfigTestState(t)

		dir := t.TempDir()
		_, err := loadConnectorFromWorkingDir(dir)
		require.EqualError(t, err, "go.mod not found from working directory")
	})

	t.Run("returns error when config file is missing", func(t *testing.T) {
		withConfigTestState(t)

		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/project\n")

		_, err := loadConnectorFromWorkingDir(dir)
		require.EqualError(t, err, ".aitestkit.json not found next to go.mod")
	})

	t.Run("returns error for invalid json", func(t *testing.T) {
		withConfigTestState(t)

		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/project\n")
		writeFile(t, filepath.Join(dir, configFileName), "{invalid")

		_, err := loadConnectorFromWorkingDir(dir)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "decode .aitestkit.json:")
	})

	t.Run("returns error for unsupported provider", func(t *testing.T) {
		withConfigTestState(t)

		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/project\n")
		writeFile(t, filepath.Join(dir, configFileName), `{"provider":"anthropic"}`)

		_, err := loadConnectorFromWorkingDir(dir)
		require.EqualError(t, err, `unsupported provider "anthropic"`)
	})

	t.Run("builds openai connector from file config", func(t *testing.T) {
		withConfigTestState(t)

		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/project\n")
		writeFile(t, filepath.Join(dir, configFileName), `{
			"provider": "openai",
			"openai": {
				"api_key_env": "OPENAI_API_KEY",
				"model": "gpt-5-mini",
				"base_url": "https://api.openai.com",
				"reasoning_effort": "minimal"
			}
		}`)

		expected := &stubConnector{}
		var captured openAIFileConfig
		buildOpenAIConnector = func(cfg openAIFileConfig) (Connector, error) {
			captured = cfg
			return expected, nil
		}

		connector, err := loadConnectorFromWorkingDir(dir)
		require.NoError(t, err)
		assert.Same(t, expected, connector)
		assert.Equal(t, openAIFileConfig{
			APIKeyEnv:       "OPENAI_API_KEY",
			Model:           "gpt-5-mini",
			BaseURL:         "https://api.openai.com",
			ReasoningEffort: "minimal",
		}, captured)
	})

	t.Run("builds runtime from inline api key", func(t *testing.T) {
		withConfigTestState(t)

		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/project\n")
		writeFile(t, filepath.Join(dir, configFileName), `{
			"provider": "openai",
			"timeout": "45s",
			"openai": {
				"api_key": "sk-inline-key",
				"model": "gpt-5-mini"
			}
		}`)

		originalBuilder := buildOpenAIConnector
		t.Cleanup(func() {
			buildOpenAIConnector = originalBuilder
		})
		buildOpenAIConnector = originalBuilder

		connector, timeout, err := loadRuntimeFromWorkingDir(dir)
		require.NoError(t, err)
		require.NotNil(t, connector)
		assert.Equal(t, 45*time.Second, timeout)
	})
}

func TestResolveAPIKey(t *testing.T) {
	t.Run("rejects missing key source", func(t *testing.T) {
		withConfigTestState(t)

		_, err := resolveAPIKey(openAIFileConfig{})
		require.EqualError(t, err, "exactly one of api_key or api_key_env must be set")
	})

	t.Run("rejects both key source options", func(t *testing.T) {
		withConfigTestState(t)

		_, err := resolveAPIKey(openAIFileConfig{
			APIKey:    "x",
			APIKeyEnv: "OPENAI_API_KEY",
		})
		require.EqualError(t, err, "exactly one of api_key or api_key_env must be set")
	})

	t.Run("returns inline key", func(t *testing.T) {
		withConfigTestState(t)

		key, err := resolveAPIKey(openAIFileConfig{APIKey: " secret "})
		require.NoError(t, err)
		assert.Equal(t, "secret", key)
	})

	t.Run("reads key from env name", func(t *testing.T) {
		withConfigTestState(t)

		lookupEnv = func(key string) (string, bool) {
			assert.Equal(t, "OPENAI_API_KEY", key)
			return " secret ", true
		}

		key, err := resolveAPIKey(openAIFileConfig{APIKeyEnv: "OPENAI_API_KEY"})
		require.NoError(t, err)
		assert.Equal(t, "secret", key)
	})

	t.Run("rejects empty env value", func(t *testing.T) {
		withConfigTestState(t)

		lookupEnv = func(_ string) (string, bool) {
			return "", true
		}

		_, err := resolveAPIKey(openAIFileConfig{APIKeyEnv: "OPENAI_API_KEY"})
		require.EqualError(t, err, `environment variable "OPENAI_API_KEY" must not be empty`)
	})
}

func TestDefaultConnectorCachesResult(t *testing.T) {
	withConfigTestState(t)

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/project\n")
	writeFile(t, filepath.Join(dir, configFileName), `{"provider":"openai","openai":{"api_key":"secret"}}`)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "internal", "tests"), 0o755))

	getWorkingDir = func() (string, error) {
		return filepath.Join(dir, "internal", "tests"), nil
	}

	expected := &stubConnector{}
	buildCalls := 0
	buildOpenAIConnector = func(cfg openAIFileConfig) (Connector, error) {
		buildCalls++
		assert.Equal(t, "secret", cfg.APIKey)
		return expected, nil
	}

	first, err := defaultConnector()
	require.NoError(t, err)
	second, err := defaultConnector()
	require.NoError(t, err)

	assert.Same(t, expected, first)
	assert.Same(t, expected, second)
	assert.Equal(t, 1, buildCalls)
}

func TestResolveTimeout(t *testing.T) {
	t.Run("uses default timeout when empty", func(t *testing.T) {
		timeout, err := resolveTimeout("")
		require.NoError(t, err)
		assert.Equal(t, defaultRequestTimeout, timeout)
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

func TestCheckResponseUsesDefaultConnector(t *testing.T) {
	withConfigTestState(t)

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/project\n")
	writeFile(t, filepath.Join(dir, configFileName), `{"provider":"openai","openai":{"api_key":"secret"}}`)
	getWorkingDir = func() (string, error) {
		return dir, nil
	}

	expected := &stubConnector{
		result: CheckResult{Score: 8, Description: "good"},
	}
	buildOpenAIConnector = func(_ openAIFileConfig) (Connector, error) {
		return expected, nil
	}

	out := &CheckResult{}
	err := CheckResponse(ResponseCheckParams{
		Subject:     "orders",
		Expectation: "must confirm success",
		Request:     map[string]string{"status": "pending"},
		Response:    map[string]string{"status": "ok"},
		MinScore:    7,
	}, out)
	require.NoError(t, err)
	assert.Equal(t, CheckResult{Score: 8, Description: "good"}, *out)
	assert.Equal(t, 1, expected.runCalls)
}

func TestAssertResponseReportsDefaultConnectorLoadError(t *testing.T) {
	withConfigTestState(t)

	getWorkingDir = func() (string, error) {
		return "", errors.New("cwd unavailable")
	}

	recorder := &recorderRequireT{}
	ok := AssertResponse(recorder, ResponseCheckParams{
		Subject:     "orders",
		Expectation: "must confirm success",
		Request:     map[string]string{"status": "pending"},
		Response:    map[string]string{"status": "ok"},
		MinScore:    7,
	})

	assert.False(t, ok)
	require.Len(t, recorder.errors, 1)
	assert.Equal(t, "orders semantic check error: load default connector: get working directory: cwd unavailable", recorder.errors[0])
}

func withConfigTestState(t *testing.T) {
	t.Helper()

	oldGetWorkingDir := getWorkingDir
	oldReadTextFile := readTextFile
	oldLookupEnv := lookupEnv
	oldBuildOpenAIConnector := buildOpenAIConnector

	resetDefaultConnectorStateForTests()

	getWorkingDir = os.Getwd
	readTextFile = os.ReadFile
	lookupEnv = os.LookupEnv
	buildOpenAIConnector = oldBuildOpenAIConnector

	t.Cleanup(func() {
		getWorkingDir = oldGetWorkingDir
		readTextFile = oldReadTextFile
		lookupEnv = oldLookupEnv
		buildOpenAIConnector = oldBuildOpenAIConnector
		resetDefaultConnectorStateForTests()
	})
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()

	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}
