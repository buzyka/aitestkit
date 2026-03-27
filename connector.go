package aitestkit

import (
	"reflect"

	"github.com/buzyka/aitestkit/internal/connectorapi"
)

// Connector executes a structured AI prompt and decodes the result into out.
type Connector = connectorapi.Connector

// PromptRequest describes a provider-agnostic structured AI request.
type PromptRequest = connectorapi.PromptRequest

// PromptPartType identifies the provider-agnostic input part kind.
type PromptPartType = connectorapi.PromptPartType

// PromptPart describes one provider-agnostic user content part.
type PromptPart = connectorapi.PromptPart

// Provider-agnostic prompt part kinds used by connectors.
const (
	PromptPartText     = connectorapi.PromptPartText
	PromptPartImageURL = connectorapi.PromptPartImageURL
)

func isNilConnector(connector Connector) bool {
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
