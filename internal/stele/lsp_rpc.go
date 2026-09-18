package stele

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// JSON-RPC 2.0 and LSP error codes.
const (
	rpcParseError           = -32700
	rpcInvalidRequest       = -32600
	rpcMethodNotFound       = -32601
	rpcInvalidParams        = -32602
	rpcServerNotInitialized = -32002
	rpcRequestFailed        = -32803
)

// rpcMessage is one JSON-RPC 2.0 message: a request when it has an ID and a
// method, a notification without an ID, and a response without a method.
type rpcMessage struct {
	ID     json.RawMessage `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

// rpcError is the error of a response.
type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (err *rpcError) Error() string {
	return err.Message
}

// newRPCError returns an error response value.
func newRPCError(code int, format string, arguments ...any) *rpcError {
	return &rpcError{Code: code, Message: fmt.Sprintf(format, arguments...)}
}

// errFraming reports a header block without a valid Content-Length.
var errFraming = errors.New("message header has no valid Content-Length")

// readFrame reads the next message body. A header block without a valid
// Content-Length returns errFraming, and the next frame can still be read.
//
// @implements req.languageserver.950acbe48c80
func readFrame(reader *bufio.Reader) ([]byte, error) {
	length := -1
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		name, value, found := strings.Cut(line, ":")
		if found && strings.EqualFold(strings.TrimSpace(name), "Content-Length") {
			if parsed, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && parsed >= 0 {
				length = parsed
			}
		}
	}
	if length < 0 {
		return nil, errFraming
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(reader, body); err != nil {
		return nil, err
	}
	return body, nil
}

// writeFrame writes one message body with its Content-Length header.
func writeFrame(writer io.Writer, body []byte) {
	_, _ = fmt.Fprintf(writer, "Content-Length: %d\r\n\r\n%s", len(body), body)
}

// lspIncoming is one frame read from the client: a body or a framing error.
type lspIncoming struct {
	body []byte
	err  error
}

// readFrames sends every frame to messages until the input ends or done
// closes, then closes messages.
func readFrames(reader *bufio.Reader, messages chan<- lspIncoming, done <-chan struct{}) {
	defer close(messages)
	for {
		body, err := readFrame(reader)
		if err != nil && !errors.Is(err, errFraming) {
			return
		}
		select {
		case messages <- lspIncoming{body: body, err: err}:
		case <-done:
			return
		}
	}
}

// responseBody builds a response around raw result bytes, which it keeps
// exactly as given.
func responseBody(id json.RawMessage, result []byte) []byte {
	return []byte(`{"jsonrpc":"2.0","id":` + string(rawOrNull(id)) + `,"result":` + string(result) + `}`)
}

// errorBody builds an error response.
func errorBody(id json.RawMessage, err *rpcError) []byte {
	content, _ := json.Marshal(err)
	return []byte(`{"jsonrpc":"2.0","id":` + string(rawOrNull(id)) + `,"error":` + string(content) + `}`)
}

// messageBody builds a request, with an ID, or a notification, without one.
func messageBody(id, method string, params any) []byte {
	content, _ := json.Marshal(params)
	if id == "" {
		return []byte(`{"jsonrpc":"2.0","method":` + strconv.Quote(method) + `,"params":` + string(content) + `}`)
	}
	return []byte(`{"jsonrpc":"2.0","id":` + strconv.Quote(id) + `,"method":` + strconv.Quote(method) +
		`,"params":` + string(content) + `}`)
}

func rawOrNull(value json.RawMessage) json.RawMessage {
	if len(value) == 0 {
		return json.RawMessage("null")
	}
	return value
}
