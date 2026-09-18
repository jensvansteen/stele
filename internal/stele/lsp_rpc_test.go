package stele

import (
	"bufio"
	"errors"
	"io"
	"strings"
	"testing"
)

// @verifies scn.languageserver.16fcd1dc9b41.unit
func TestLSPSurvivesMalformedInput(t *testing.T) {
	root := lspFixture(t)
	client := startLSP(t, neverFires)
	client.sendRaw(`{"jsonrpc":"2.0","id":1,"method":`)
	parseError := client.response("null")
	if parseError.Error == nil || parseError.Error.Code != rpcParseError {
		t.Fatalf("malformed message = %#v", parseError)
	}
	var initialized struct {
		ServerInfo struct {
			Name string `json:"name"`
		} `json:"serverInfo"`
	}
	client.result(client.initializeLSP(root, lspCapabilities()), &initialized)
	if initialized.ServerInfo.Name != "stele" {
		t.Fatalf("initialize after a malformed message = %#v", initialized)
	}
	_, _ = io.WriteString(client.input, "X-Other: 1\r\n\r\n")
	if framing := client.response("null"); framing.Error == nil || framing.Error.Code != rpcParseError {
		t.Fatalf("frame without Content-Length = %#v", framing)
	}
	if hover := client.request("textDocument/hover", position(root, "src/demo.mts", 0, 20)); hover.Error != nil ||
		!strings.Contains(string(hover.Result), "Return value") {
		t.Fatalf("hover after malformed frames = %#v", hover)
	}
}

func TestReadFrame(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("content-length: 2\r\nContent-Type: x\r\n\r\n{}" +
		"Content-Length: nope\r\n\r\n" + "Content-Length: 5\r\n\r\n{}"))
	if body, err := readFrame(reader); err != nil || string(body) != "{}" {
		t.Fatalf("frame = %q, %v", body, err)
	}
	if _, err := readFrame(reader); !errors.Is(err, errFraming) {
		t.Fatalf("invalid length = %v", err)
	}
	if _, err := readFrame(reader); err == nil {
		t.Fatal("a short body was accepted")
	}
	if _, err := readFrame(bufio.NewReader(strings.NewReader("Content-Length: 1"))); err == nil {
		t.Fatal("an unterminated header was accepted")
	}
}

func TestReadFramesStopsWhenDone(t *testing.T) {
	messages := make(chan lspIncoming)
	done := make(chan struct{})
	close(done)
	readFrames(bufio.NewReader(strings.NewReader("Content-Length: 2\r\n\r\n{}")), messages, done)
	if _, open := <-messages; open {
		t.Fatal("readFrames sent a frame after done")
	}
}

func TestRPCBodies(t *testing.T) {
	if body := string(responseBody(nil, []byte("1"))); body != `{"jsonrpc":"2.0","id":null,"result":1}` {
		t.Fatalf("response = %s", body)
	}
	if body := string(errorBody([]byte(`"a"`), newRPCError(rpcRequestFailed, "failed %d", 1))); body !=
		`{"jsonrpc":"2.0","id":"a","error":{"code":-32803,"message":"failed 1"}}` {
		t.Fatalf("error = %s", body)
	}
	if message := newRPCError(rpcInvalidParams, "bad").Error(); message != "bad" {
		t.Fatalf("error text = %q", message)
	}
	want := `{"jsonrpc":"2.0","id":"stele-1","method":"m","params":null}`
	if body := string(messageBody("stele-1", "m", nil)); body != want {
		t.Fatalf("request = %s", body)
	}
}
