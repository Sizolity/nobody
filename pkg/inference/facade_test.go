package inference_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sizolity/nobody/pkg/inference"
)

func TestPublicInferenceConstantsAndState(t *testing.T) {
	require.Equal(t, "connected", inference.StateConnected.String())
	require.Equal(t, "disconnected", inference.StateDisconnected.String())
	require.Equal(t, "inference_check", inference.EventInferenceCheck)
	require.Equal(t, "provider", inference.PayloadProviderKey)
}

func TestPublicEventEmitterType(t *testing.T) {
	var got []string
	var emit inference.EventEmitter = func(eventName, severity string, payload map[string]any) {
		got = append(got, eventName, severity, payload[inference.PayloadProviderKey].(string))
	}

	emit(inference.EventInferenceCheck, "info", map[string]any{
		inference.PayloadProviderKey: "llamacpp",
	})

	require.Equal(t, []string{"inference_check", "info", "llamacpp"}, got)
}
