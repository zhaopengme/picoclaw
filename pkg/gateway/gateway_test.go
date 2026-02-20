package gateway

import (
	"testing"
)

func TestGatewayRoutesCommand(t *testing.T) {
	g := NewCommandGateway(nil, nil, nil)
	if g == nil {
		t.Fatal("expected gateway")
	}
}
