// Package runtimeconfig loads and caches the semantic check runtime from
// `.aitestkit.json` placed next to the module `go.mod`.
package runtimeconfig

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/buzyka/aitestkit/internal/connectorapi"
	"github.com/buzyka/aitestkit/openai"
)

const (
	// ConfigFileName is the runtime configuration file name expected next to `go.mod`.
	ConfigFileName = ".aitestkit.json"
	// DefaultRequestTimeout is used when the configuration file does not define `timeout`.
	DefaultRequestTimeout = 5 * time.Minute
)

// Runtime contains the lazily loaded connector and timeout used by semantic checks.
type Runtime struct {
	Connector connectorapi.Connector
	Timeout   time.Duration
}

// FileConfig is the `.aitestkit.json` structure.
type FileConfig struct {
	Provider string       `json:"provider"`
	Timeout  string       `json:"timeout"`
	OpenAI   OpenAIConfig `json:"openai"`
}

// OpenAIConfig contains provider-specific configuration for the OpenAI connector.
type OpenAIConfig struct {
	APIKey          string `json:"api_key"`
	APIKeyEnv       string `json:"api_key_env"`
	Model           string `json:"model"`
	BaseURL         string `json:"base_url"`
	ReasoningEffort string `json:"reasoning_effort"`
}

// TestHooks replace environment-dependent functions during tests.
type TestHooks struct {
	GetWorkingDir        func() (string, error)
	ReadTextFile         func(string) ([]byte, error)
	LookupEnv            func(string) (string, bool)
	BuildOpenAIConnector func(OpenAIConfig) (connectorapi.Connector, error)
}

var (
	defaultRuntimeOnce  sync.Once
	defaultRuntimeValue Runtime
	defaultRuntimeErr   error

	getWorkingDir        = os.Getwd
	readTextFile         = os.ReadFile
	lookupEnv            = os.LookupEnv
	buildOpenAIConnector = defaultBuildOpenAIConnector
)

// DefaultRuntime returns the cached runtime built from `.aitestkit.json`.
func DefaultRuntime() (Runtime, error) {
	defaultRuntimeOnce.Do(func() {
		workingDir, err := getWorkingDir()
		if err != nil {
			defaultRuntimeErr = fmt.Errorf("get working directory: %w", err)
			return
		}

		defaultRuntimeValue, defaultRuntimeErr = LoadRuntimeFromWorkingDir(workingDir)
	})

	if defaultRuntimeErr != nil {
		return Runtime{}, defaultRuntimeErr
	}

	if isNilConnector(defaultRuntimeValue.Connector) {
		return Runtime{}, errors.New("default connector is required")
	}

	return defaultRuntimeValue, nil
}

// LoadRuntimeFromWorkingDir resolves `.aitestkit.json` near the closest `go.mod`.
func LoadRuntimeFromWorkingDir(startDir string) (Runtime, error) {
	moduleRoot, err := findModuleRoot(startDir)
	if err != nil {
		return Runtime{}, err
	}

	cfg, err := loadFileConfig(filepath.Join(moduleRoot, ConfigFileName))
	if err != nil {
		return Runtime{}, err
	}

	timeout, err := resolveTimeout(cfg.Timeout)
	if err != nil {
		return Runtime{}, err
	}

	switch strings.TrimSpace(cfg.Provider) {
	case "openai":
		connector, buildErr := buildOpenAIConnector(cfg.OpenAI)
		if buildErr != nil {
			return Runtime{}, fmt.Errorf("build openai connector: %w", buildErr)
		}
		return Runtime{
			Connector: connector,
			Timeout:   timeout,
		}, nil
	case "":
		return Runtime{}, errors.New("config provider must not be empty")
	default:
		return Runtime{}, fmt.Errorf("unsupported provider %q", cfg.Provider)
	}
}

// ResetForTests clears the cached runtime and restores default hooks.
func ResetForTests() {
	defaultRuntimeOnce = sync.Once{}
	defaultRuntimeValue = Runtime{}
	defaultRuntimeErr = nil
	getWorkingDir = os.Getwd
	readTextFile = os.ReadFile
	lookupEnv = os.LookupEnv
	buildOpenAIConnector = defaultBuildOpenAIConnector
}

// SetTestHooks overrides environment-dependent hooks for tests.
func SetTestHooks(hooks TestHooks) {
	if hooks.GetWorkingDir != nil {
		getWorkingDir = hooks.GetWorkingDir
	}
	if hooks.ReadTextFile != nil {
		readTextFile = hooks.ReadTextFile
	}
	if hooks.LookupEnv != nil {
		lookupEnv = hooks.LookupEnv
	}
	if hooks.BuildOpenAIConnector != nil {
		buildOpenAIConnector = hooks.BuildOpenAIConnector
	}
}

// SetDefaultRuntimeForTests injects a cached runtime for tests.
func SetDefaultRuntimeForTests(runtime Runtime, err error) {
	defaultRuntimeOnce = sync.Once{}
	defaultRuntimeOnce.Do(func() {
		defaultRuntimeValue = runtime
		defaultRuntimeErr = err
	})
}

func defaultBuildOpenAIConnector(cfg OpenAIConfig) (connectorapi.Connector, error) {
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

func loadFileConfig(path string) (FileConfig, error) {
	content, err := readTextFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return FileConfig{}, fmt.Errorf("%s not found next to go.mod", ConfigFileName)
		}
		return FileConfig{}, fmt.Errorf("read %s: %w", ConfigFileName, err)
	}

	var cfg FileConfig
	if err := json.Unmarshal(content, &cfg); err != nil {
		return FileConfig{}, fmt.Errorf("decode %s: %w", ConfigFileName, err)
	}

	return cfg, nil
}

func resolveAPIKey(cfg OpenAIConfig) (string, error) {
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
		return DefaultRequestTimeout, nil
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

func isNilConnector(connector connectorapi.Connector) bool {
	if connector == nil {
		return true
	}

	value := reflect.ValueOf(connector)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}
