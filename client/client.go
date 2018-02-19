package webwire_client

import (
	webwire "github.com/qbeon/webwire-go"

	"io"
	"log"
	"fmt"
	"time"
	"bytes"
	"strings"
	"net/url"
	"net/http"
	"io/ioutil"
	"encoding/json"
	"github.com/satori/go.uuid"
	"github.com/gorilla/websocket"
)

const supportedProtocolVersion = "1.0"

// OnServerSignal is an optional callback.
// It's invoked when the webwire client receives a signal from the server
type OnServerSignal func([]byte)

// OnSessionCreated is an optional callback.
// It's invoked when the webwire client receives a new session
type OnSessionCreated func(*webwire.Session)

// OnSessionClosed is an optional callback.
// It's invoked when the clients session was closed either by the server or by himself
type OnSessionClosed func()

func extractMessageId(message []byte) (arr [32]byte) {
	copy(arr[:], message[1:33])
	return arr
}

type Client struct {
	serverAddr string
	defaultTimeout time.Duration
	conn *websocket.Conn
	reqRegister map[[32]byte] chan []byte
	sess *webwire.Session

	// Handlers
	onServerSignal OnServerSignal
	onSessionCreated OnSessionCreated
	onSessionClosed OnSessionClosed

	// Loggers
	warningLog *log.Logger
	errorLog *log.Logger
}

// NewClient creates a new disconnected client instance. 
func NewClient(
	serverAddr string,
	onServerSignal OnServerSignal,
	onSessionCreated OnSessionCreated,
	onSessionClosed OnSessionClosed,
	defaultTimeout time.Duration,
	warningLogWriter io.Writer,
	errorLogWriter io.Writer,
) Client {
	if onServerSignal == nil {
		onServerSignal = func(_ []byte) {}
	}

	if onSessionCreated == nil {
		onSessionCreated = func(_ *webwire.Session) {}
	}

	if onSessionClosed == nil {
		onSessionClosed = func() {}
	}

	return Client {
		serverAddr,
		defaultTimeout,
		nil,
		make(map[[32]byte] chan []byte, 0),
		nil,
		onServerSignal,
		onSessionCreated,
		onSessionClosed,
		log.New(
			warningLogWriter,
			"WARNING: ",
			log.Ldate | log.Ltime | log.Lshortfile,
		),
		log.New(
			errorLogWriter,
			"ERROR: ",
			log.Ldate | log.Ltime | log.Lshortfile,
		),
	}
}

func (clt *Client) onRequest(payload []byte) ([]byte, error) {
	// TODO: implement real server-request handling
	// instead of current ping-pong
	return payload, nil
}

func (clt *Client) handleRequest(message []byte) error {
	reqId := extractMessageId(message)
	// Handle server request
	result, err := clt.onRequest(message[33:])
	var msg bytes.Buffer
	if err != nil {
		msg.WriteRune(webwire.MsgTyp_ERROR_RESP)
		msg.Write(reqId[:])
		msg.WriteString(err.Error())
	} else {
		msg.WriteRune(webwire.MsgTyp_RESPONSE)
		msg.Write(reqId[:])
		msg.Write(result)
	}
	if err = clt.conn.WriteMessage(websocket.TextMessage, msg.Bytes());
	err != nil {
		// TODO: return typed error TransmissionFailure
		return fmt.Errorf("Couldn't send message")
	}
	return nil
}

func (clt *Client) handleSessionCreated(message []byte) error {
	// Set new session
	// TODO: get session creation time from actual server time
	// TODO: get session info from appended JSON
	clt.sess = &webwire.Session {
		Key: string(message),
		CreationDate: time.Now(),
	}

	clt.onSessionCreated(clt.sess)

	return nil
}

func (clt *Client) handleSessionClosed() error {
	// Destroy local session
	clt.sess = nil

	clt.onSessionClosed()

	return nil
}

func (clt *Client) handleFailure(message []byte) error {
	return nil
}

func (clt *Client) handleResponse(message []byte) error {
	reqId := extractMessageId(message)

	if response, exists := clt.reqRegister[reqId]; exists {
		// Fulfill response
		response <- message[33:]
		delete(clt.reqRegister, reqId)
	}

	return nil
}

func (clt *Client) handleMessage(message []byte) error {
	if len(message) < 1 {
		return nil
	}
	switch (message[0:1][0]) {
	case webwire.MsgTyp_RESPONSE: return clt.handleResponse(message)
	case webwire.MsgTyp_ERROR_RESP: return clt.handleFailure(message)
	case webwire.MsgTyp_SIGNAL:
		clt.onServerSignal(message[1:])
		return nil
	case webwire.MsgTyp_REQUEST: return clt.handleRequest(message)
	case webwire.MsgTyp_SESS_CREATED: return clt.handleSessionCreated(message[1:])
	case webwire.MsgTyp_SESS_CLOSED: return clt.handleSessionClosed()
	// TODO: write warning to warningLog
	// TODO: write warning to warningLog
	default: fmt.Printf("Strange message type received: '%c'\n", message[0:1][0])
	}
	return nil
}
// verifyProtocolVersion requests the endpoint metadata
// to verify the server is running a supported protocol version
func (clt *Client) verifyProtocolVersion() error {
	// Initialize HTTP client
	var httpClient = &http.Client {
		Timeout: time.Second * 10,
	}

	request, err := http.NewRequest("WEBWIRE", "http://" + clt.serverAddr + "/", nil)
	if err != nil {
		return fmt.Errorf("Couldn't create HTTP metadata request: %s", err)
	}
	response, err := httpClient.Do(request)
	if err != nil {
		fmt.Errorf("Endpoint metadata request failed: %s", err)
	}

	// Read response body
	defer response.Body.Close()
	encodedData, err := ioutil.ReadAll(response.Body)
	if err != nil {
		fmt.Errorf("Couldn't read metadata response body: %s", err)
	}

	// Unmarshal response
	var metadata struct {
		ProtocolVersion string `json:"protocol-version"`
	}
	if err := json.Unmarshal(encodedData, &metadata); err != nil {
		fmt.Errorf("Couldn't parse HTTP metadata response ('%s'): %s", string(encodedData), err)
	}

	// Verify metadata
	if metadata.ProtocolVersion != supportedProtocolVersion {
		fmt.Errorf(
			"Unsupported protocol version: %s (%s is supported by this client)",
			metadata.ProtocolVersion,
			supportedProtocolVersion,
		)
	}

	return nil
}


// Connect connects the client to the configured server and
// returns an error in case of a connection failure
func (clt *Client) Connect() (err error) {
	if clt.conn != nil {
		return nil
	}

	if err := clt.verifyProtocolVersion(); err != nil {
		return err
	}

	connUrl := url.URL {Scheme: "ws", Host: clt.serverAddr, Path: "/"}
	clt.conn, _, err = websocket.DefaultDialer.Dial(connUrl.String(), nil)
	if err != nil {
		// TODO: return typed error ConnectionFailure
		return fmt.Errorf("Could not connect: %s", err)
	}

	// Setup reader thread
	go func() {
		defer clt.Close()
		for {
			_, message, err := clt.conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(
					err,
					websocket.CloseGoingAway,
					websocket.CloseAbnormalClosure,
				) {
					// Error while reading message
					clt.errorLog.Print("Failed reading message:", err)
					break
				} else {
					// Shutdown client due to clean disconnection
					break
				}
			}
			// Try to handle the message
			if err = clt.handleMessage(message); err != nil {
				clt.warningLog.Print("Failed handling message:", err)
			}
		}
	}()

	return nil
}

func (clt *Client) sendRequest(
	messageType rune,
	payload []byte,
	timeout time.Duration,
) ([]byte, error) {
	// Connect before attempting to send the request
	if err := clt.Connect(); err != nil {
		return nil, fmt.Errorf("Couldn't connect: %s", err)
	}

	id := uuid.NewV4()
	var reqId [32]byte
	copy(reqId[:], strings.Replace(id.String(), "-", "", -1))
	var msg bytes.Buffer
	msg.WriteRune(messageType)
	msg.Write(reqId[:])
	msg.Write(payload)

	timeoutTimer := time.NewTimer(timeout).C
	responseChannel := make(chan []byte)

	// Register request
	clt.reqRegister[reqId] = responseChannel

	// Send request
	if err := clt.conn.WriteMessage(websocket.TextMessage, msg.Bytes()); err != nil {
		// TODO: return typed error TransmissionFailure
		return nil, fmt.Errorf("Couldn't send message: %s", err)
	}

	// Block until request either times out or a response is received
	select {
	case <- timeoutTimer:
		// TODO: return typed TimeoutError
		return nil, fmt.Errorf("Request timed out")
	case response := <- responseChannel:
		return response, nil
	}
}

// Request sends a request containing the given payload to the server
// and asynchronously returns the servers response
// blocking the calling goroutine.
// Returns an error if the request failed for some reason.
// Attempts to automatically connect to the server
// if no connection has yet been established
func (clt *Client) Request(payload []byte) ([]byte, error) {
	return clt.sendRequest(webwire.MsgTyp_REQUEST, payload, clt.defaultTimeout)
}

// TimedRequest sends a request containing the given payload to the server
// and asynchronously returns the servers response
// blocking the calling goroutine.
// Returns an error if the given timeout was exceeded awaiting the response
// ar another failure occurred.
// Attempts to automatically connect to the server
// if no connection has yet been established
func (clt *Client) TimedRequest(payload []byte, timeout time.Duration) ([]byte, error) {
	return clt.sendRequest(webwire.MsgTyp_REQUEST, payload, timeout)
}

// Signal sends a signal containing the given payload to the server.
// Attempts to automatically connect to the server
// if no connection has yet been established
func (clt *Client) Signal(payload []byte) error {
	// Connect before attempting to send the signal
	if err := clt.Connect(); err != nil {
		return fmt.Errorf("Couldn't connect to server")
	}

	var msg bytes.Buffer
	msg.WriteRune(webwire.MsgTyp_SIGNAL)
	msg.Write(payload)
	if err := clt.conn.WriteMessage(websocket.TextMessage, msg.Bytes());
	err != nil {
		return err
	}
	return nil
}

// Session returns information about the current session
func (clt *Client) Session() webwire.Session {
	if clt.sess == nil {
		return webwire.Session {}
	}
	return *clt.sess
}

// RestoreSession tries to restore the previously opened session
// Fails if a session is currently already active
// Attempts to automatically connect to the server
// if no connection has yet been established
func (clt *Client) RestoreSession(sessionKey []byte) error {
	// Connect before attempting session restoration
	if err := clt.Connect(); err != nil {
		return fmt.Errorf("Couldn't connect: %s", err)
	}

	if _, err := clt.sendRequest(webwire.MsgTyp_SESS_RESTORE, sessionKey, clt.defaultTimeout);
	err != nil {
		// TODO: check for error types
		return fmt.Errorf("Session restoration request failed: %s", err)
	}
	
	return nil
}

// CloseSession closes the currently active session.
// Does nothing if there's no active session
func (clt *Client) CloseSession() error {
	if clt.conn == nil {
		return fmt.Errorf("Cannot close a session of a disconnected client")
	}

	if clt.sess == nil {
		return nil
	}

	if _, err := clt.sendRequest(webwire.MsgTyp_CLOSE_SESSION, nil, clt.defaultTimeout);
	err != nil {
		return fmt.Errorf("Session destruction request failed: %s", err)
	}

	// Reset session locally after destroying it on the server
	clt.sess = nil

	return nil
}

// Close gracefully closes the connection.
// Does nothing if the client isn't connected
func (clt *Client) Close() {
	if clt.conn == nil {
		return
	}
	clt.conn.Close()
}
