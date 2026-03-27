package aitestkit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/buzyka/aitestkit/openai"
)

const (
	configFileName        = ".aitestkit.json"
	defaultRequestTimeout = 5 * time.Minute
)

type fileConfig struct {
	Provider string           `json:"provider"`
	Timeout  string           `json:"timeout"`
	OpenAI   openAIFileConfig `json:"openai"`
}

type openAIFileConfig struct {
	APIKey          string `json:"api_key"`
	APIKeyEnv       string `json:"api_key_env"`
	Model           string `json:"model"`
	BaseURL         string `json:"base_url"`
	ReasoningEffort string `json:"reasoning_effort"`
}

var (
	defaultConnectorOnce  sync.Once
	defaultConnectorValue Connector
	defaultTimeoutValue   time.Duration
	defaultConnectorErr   error

	getWorkingDir = os.Getwd
	readTextFile  = os.ReadFile
	lookupEnv     = os.LookupEnv

	buildOpenAIConnector = func(cfg openAIFileConfig) (Connector, error) {
		apiKey, err := resolveAPIKey(cfg)
		if err != nil {
			return nil, err
		}

		opts := make([]openai.Option, 0, 3)
		if strings.TrimSpace(cfg.Model) != "" {
			opts = append(opts, openai.WithModel(cfg.Model))
		}
		if strings.TrimSpace(cfg.BaseURL) != "" {
			opts = append(opts, openai.WithBaseURL(cfg.BaseURL))
		}
		if strings.TrimSpace(cfg.ReasoningEffort) != "" {
			opts = append(opts, openai.WithReasoningEffort(cfg.ReasoningEffort))
		}

		return openai.NewConnector(apiKey, opts...)
	}
)

func defaultConnector() (Connector, error) {
	defaultConnectorOnce.Do(func() {
		workingDir, err := getWorkingDir()
		if err != nil {
			defaultConnectorErr = fmt.Errorf("get working directory: %w", err)
			return
		}

		defaultConnectorValue, defaultTimeoutValue, defaultConnectorErr = loadRuntimeFromWorkingDir(workingDir)
	})

	if isNilConnector(defaultConnectorValue) {
		return nil, defaultConnectorErr
	}

	return defaultConnectorValue, defaultConnectorErr
}

func defaultTimeout() (time.Duration, error) {
	defaultConnectorOnce.Do(func() {
		workingDir, err := getWorkingDir()
		if err != nil {
			defaultConnectorErr = fmt.Errorf("get working directory: %w", err)
			return
		}

		defaultConnectorValue, defaultTimeoutValue, defaultConnectorErr = loadRuntimeFromWorkingDir(workingDir)
	})

	if defaultConnectorErr != nil {
		return 0, defaultConnectorErr
	}

	return defaultTimeoutValue, nil
}

func loadConnectorFromWorkingDir(startDir string) (Connector, error) {
	connector, _, err := loadRuntimeFromWorkingDir(startDir)
	return connector, err
}

func loadRuntimeFromWorkingDir(startDir string) (Connector, time.Duration, error) {
	moduleRoot, err := findModuleRoot(startDir)
	if err != nil {
		return nil, 0, err
	}

	cfg, err := loadFileConfig(filepath.Join(moduleRoot, configFileName))
	if err != nil {
		return nil, 0, err
	}

	timeout, err := resolveTimeout(cfg.Timeout)
	if err != nil {
		return nil, 0, err
	}

	switch strings.TrimSpace(cfg.Provider) {
	case "openai":
		connector, buildErr := buildOpenAIConnector(cfg.OpenAI)
		if buildErr != nil {
			return nil, 0, fmt.Errorf("build openai connector: %w", buildErr)
		}
		return connector, timeout, nil
	case "":
		return nil, 0, errors.New("config provider must not be empty")
	default:
		return nil, 0, fmt.Errorf("unsupported provider %q", cfg.Provider)
	}
}

func findModuleRoot(startDir string) (string, error) {
	dir := filepath.Clean(startDir)

	for {
		goModPath := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("go.mod not found from working directory")
		}

		dir = parent
	}
}

func loadFileConfig(path string) (fileConfig, error) {
	content, err := readTextFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fileConfig{}, fmt.Errorf("%s not found next to go.mod", configFileName)
		}
		return fileConfig{}, fmt.Errorf("read %s: %w", configFileName, err)
	}

	var cfg fileConfig
	if err := json.Unmarshal(content, &cfg); err != nil {
		return fileConfig{}, fmt.Errorf("decode %s: %w", configFileName, err)
	}

	return cfg, nil
}

func resolveAPIKey(cfg openAIFileConfig) (string, error) {
	inlineKey := strings.TrimSpace(cfg.APIKey)
	envName := strings.TrimSpace(cfg.APIKeyEnv)

	switch {
	case inlineKey != "" && envName != "":
		return "", errors.New("exactly one of api_key or api_key_env must be set")
	case inlineKey != "":
		return inlineKey, nil
	case envName != "":
		value, ok := lookupEnv(envName)
		if !ok || strings.TrimSpace(value) == "" {
			return "", fmt.Errorf("environment variable %q must not be empty", envName)
		}
		return strings.TrimSpace(value), nil
	default:
		return "", errors.New("exactly one of api_key or api_key_env must be set")
	}
}

func resolveTimeout(raw string) (time.Duration, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return defaultRequestTimeout, nil
	}

	timeout, err := time.ParseDuration(trimmed)
	if err != nil {
		return 0, fmt.Errorf("parse timeout: %w", err)
	}

	if timeout <= 0 {
		return 0, errors.New("timeout must be greater than zero")
	}

	return timeout, nil
}

func resetDefaultConnectorStateForTests() {
	defaultConnectorOnce = sync.Once{}
	defaultConnectorValue = nil
	defaultTimeoutValue = 0
	defaultConnectorErr = nil
}

func checkResponseWithConnector(ctx context.Context, c Connector, params ResponseCheckParams, out *CheckResult) error {
	if isNilConnector(c) {
		return errors.New("connector is required")
	}

	return executeResponseCheck(ctx, c, params, out)
}

func checkImageResponseWithConnector(ctx context.Context, c Connector, params ImageResponseCheckParams, out *CheckResult) error {
	if isNilConnector(c) {
		return errors.New("connector is required")
	}

	return executeImageResponseCheck(ctx, c, params, out)
}
