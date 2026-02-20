package bus

import (
	"testing"
)

func TestMessageBusImplementsBroker(t *testing.T) {
	var _ Broker = (*MessageBus)(nil)
}
