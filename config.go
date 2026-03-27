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

	"github.com/buzyka/aitestkit/openai"
)

const configFileName = ".aitestkit.json"

type fileConfig struct {
	Provider string           `json:"provider"`
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

		defaultConnectorValue, defaultConnectorErr = loadConnectorFromWorkingDir(workingDir)
	})

	if isNilConnector(defaultConnectorValue) {
		return nil, defaultConnectorErr
	}

	return defaultConnectorValue, defaultConnectorErr
}

func loadConnectorFromWorkingDir(startDir string) (Connector, error) {
	moduleRoot, err := findModuleRoot(startDir)
	if err != nil {
		return nil, err
	}

	cfg, err := loadFileConfig(filepath.Join(moduleRoot, configFileName))
	if err != nil {
		return nil, err
	}

	switch strings.TrimSpace(cfg.Provider) {
	case "openai":
		connector, buildErr := buildOpenAIConnector(cfg.OpenAI)
		if buildErr != nil {
			return nil, fmt.Errorf("build openai connector: %w", buildErr)
		}
		return connector, nil
	case "":
		return nil, errors.New("config provider must not be empty")
	default:
		return nil, fmt.Errorf("unsupported provider %q", cfg.Provider)
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

func resetDefaultConnectorStateForTests() {
	defaultConnectorOnce = sync.Once{}
	defaultConnectorValue = nil
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
