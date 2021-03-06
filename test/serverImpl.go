package test

import (
	"context"
	"net/http"

	wwr "github.com/qbeon/webwire-go"
)

// serverImpl implements the webwire.ServerImplementation interface
type serverImpl struct {
	beforeUpgrade func(
		resp http.ResponseWriter,
		req *http.Request,
	) wwr.ConnectionOptions
	onClientConnected    func(connection wwr.Connection)
	onClientDisconnected func(connection wwr.Connection)
	onSignal             func(
		ctx context.Context,
		connection wwr.Connection,
		message wwr.Message,
	)
	onRequest func(
		ctx context.Context,
		connection wwr.Connection,
		message wwr.Message,
	) (response wwr.Payload, err error)
}

// OnOptions implements the webwire.ServerImplementation interface
func (srv *serverImpl) OnOptions(resp http.ResponseWriter) {
	resp.Header().Set("Access-Control-Allow-Origin", "*")
	resp.Header().Set("Access-Control-Allow-Methods", "WEBWIRE")
}

// BeforeUpgrade implements the webwire.ServerImplementation interface
func (srv *serverImpl) BeforeUpgrade(
	resp http.ResponseWriter,
	req *http.Request,
) wwr.ConnectionOptions {
	return srv.beforeUpgrade(resp, req)
}

// OnClientConnected implements the webwire.ServerImplementation interface
func (srv *serverImpl) OnClientConnected(conn wwr.Connection) {
	srv.onClientConnected(conn)
}

// OnClientDisconnected implements the webwire.ServerImplementation interface
func (srv *serverImpl) OnClientDisconnected(conn wwr.Connection) {
	srv.onClientDisconnected(conn)
}

// OnSignal implements the webwire.ServerImplementation interface
func (srv *serverImpl) OnSignal(
	ctx context.Context,
	clt wwr.Connection,
	msg wwr.Message,
) {
	srv.onSignal(ctx, clt, msg)
}

// OnRequest implements the webwire.ServerImplementation interface
func (srv *serverImpl) OnRequest(
	ctx context.Context,
	clt wwr.Connection,
	msg wwr.Message,
) (response wwr.Payload, err error) {
	return srv.onRequest(ctx, clt, msg)
}
