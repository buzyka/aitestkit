package aitestkit

import (
	"time"

	"github.com/buzyka/aitestkit/internal/runtimeconfig"
)

func defaultConnector() (Connector, error) {
	runtime, err := runtimeconfig.DefaultRuntime()
	if err != nil {
		return nil, err
	}

	return runtime.Connector, nil
}

func defaultTimeout() (time.Duration, error) {
	runtime, err := runtimeconfig.DefaultRuntime()
	if err != nil {
		return 0, err
	}

	return runtime.Timeout, nil
}

func resetDefaultConnectorStateForTests() {
	runtimeconfig.ResetForTests()
}
