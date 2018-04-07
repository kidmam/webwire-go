package test

import (
	"reflect"
	"testing"
	"time"

	wwr "github.com/qbeon/webwire-go"
	wwrclt "github.com/qbeon/webwire-go/client"
)

// TestClientSignalDisconnected tests sending signals on disconnected clients
func TestClientSignalDisconnected(t *testing.T) {
	// Initialize webwire server given only the request
	server := setupServer(
		t,
		&serverImpl{},
		wwr.ServerOptions{},
	)

	// Initialize client and skip manual connection establishment
	client := newCallbackPoweredClient(
		server.Addr().String(),
		wwrclt.Options{
			DefaultRequestTimeout: 2 * time.Second,
		},
		nil, nil, nil, nil,
	)

	// Send request and await reply
	if err := client.connection.Signal(
		"",
		wwr.Payload{Data: []byte("testdata")},
	); err != nil {
		t.Fatalf(
			"Expected signal to automatically connect, got error: %s | %s",
			reflect.TypeOf(err),
			err,
		)
	}
}
