package sockets

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"gopkg.in/macaron.v1"
	"github.com/gorilla/websocket"
)

const (
	host              string = "http://localhost:4000"
	endpoint          string = "ws://localhost:4000"
	recvPath          string = "/receiver"
	sendPath          string = "/sender"
	pingPath          string = "/ping"
	recvStringsPath   string = "/strings/receiver"
	recvByteSlicePath string = "/byteslice/receiver"
	sendStringsPath   string = "/strings/sender"
	sendByteSlicePath string = "/byteslice/sender"
	pingStringsPath   string = "/strings/ping"
	crossOriginPath   string = "/cross/origin"
)

type Message struct {
	Text string `json:"text"`
}

var (
	once                  sync.Once
	recvMessages          []*Message
	recvByteSliceMessages []byte
	recvCount             int
	recvDone              bool
	sendMessages          []*Message
	sendCount             int
	sendDone              bool
	recvStrings           []string
	recvByteSlices        [][]byte
	recvStringsCount      int
	recvByteSliceCount    int
	recvStringsDone       bool
	recvByteSlicesDone    bool
	sendStrings           []string
	sendByteSlices        [][]byte
	sendStringsCount      int
	sendStringsDone       bool
	sendByteSlicesDone    bool
)

// Test Helpers
func expectSame(t *testing.T, a interface{}, b interface{}) {
	if a != b {
		t.Errorf("Expected %T: %v to be %T: %v", b, b, a, a)
	}
}

func expectStringsToBeEmpty(t *testing.T, strings []string) {
	if len(strings) > 0 {
		t.Errorf("Expected strings array to be empty, but they contained %d values", len(strings))
	}
}

func expectByteSlicesToBeEmpty(t *testing.T, data [][]byte) {
	if len(data) > 0 {
		t.Errorf("Expected ByteSlice arrays to be empty, but they contained %d values", len(data))
	}
}

func expectMessagesToBeEmpty(t *testing.T, messages []*Message) {
	if len(messages) > 0 {
		t.Errorf("Expected messages array to be empty, but they contained %d values", len(messages))
	}
}

func expectByteSliceMessagesToBeEmpty(t *testing.T, messages []byte) {
	if len(messages) > 0 {
		t.Errorf("Expected messages array to be empty, but they contained %d values", len(messages))
	}
}

func expectStringsToHaveArrived(t *testing.T, count int, strings []string) {
	if len(strings) < count {
		t.Errorf("Expected strings array to contain 3 values, but contained %d", len(strings))
	} else {
		for i, s := range strings {
			if s != "Hello World" {
				t.Errorf("Expected string %d to be \"Hello World\", but was \"%v\"", i+1, s)
			}
		}
	}
}

func expectByteSlicesToHaveArrived(t *testing.T, count int, byteSlices [][]byte) {
	if len(byteSlices) < count {
		t.Errorf("Expected ByteSlice array to contain 3 values, but contained %d", len(byteSlices))
	} else {
		for i, s := range byteSlices {
			if string(s) != "Hello World" {
				t.Errorf("Expected ByteSlice %d to be \"Hello World\", but was \"%v\"", i+1, string(s))
			}
		}
	}
}

func expectMessagesToHaveArrived(t *testing.T, count int, messages []*Message) {
	if len(messages) < count {
		t.Errorf("Expected messages array to contain 3 values, but contained %d", len(messages))
	} else {
		for i, m := range messages {
			if m.Text != "Hello World" {
				t.Errorf("Expected message %d to contain \"Hello World\", but contained %v", i+1, m)
			}
		}
	}
}

func expectPingsToHaveBeenExecuted(t *testing.T, count int, messages []*Message) {
	if len(messages) < count {
		t.Errorf("Expected messages array to contain 3 ping values, but contained %d", len(messages))
	} else {
		for i, m := range messages {
			if m.Text != "" {
				t.Errorf("Expected message %d to contain \"\", but contained %v", i+1, m)
			}
		}
	}
}

func expectStatusCode(t *testing.T, expectedStatusCode int, actualStatusCode int) {
	if actualStatusCode != expectedStatusCode {
		t.Errorf("Expected StatusCode %d, but received %d", expectedStatusCode, actualStatusCode)
	}
}

func expectIsDone(t *testing.T, done bool) {
	if !done {
		t.Errorf("Expected to be done, but was not")
	}
}

func startServer() {
	m := macaron.Classic()

	m.Get(recvPath, JSON(Message{}), func(context *macaron.Context, receiver <-chan *Message, done <-chan bool) int {
		for {
			select {
			case msg := <-receiver:
				recvMessages = append(recvMessages, msg)
			case <-done:
				recvDone = true
				return http.StatusOK
			}
		}

		return http.StatusOK
	})

	m.Get(sendPath, JSON(Message{}), func(context *macaron.Context, sender chan<- *Message, done <-chan bool, disconnect chan<- int) int {
		ticker := time.NewTicker(1 * time.Millisecond)
		bomb := time.After(4 * time.Millisecond)

		for {
			select {
			case <-ticker.C:
				sender <- &Message{"Hello World"}
			case <-done:
				ticker.Stop()
				sendDone = true
				return http.StatusOK
			case <-bomb:
				disconnect <- websocket.CloseGoingAway
				return http.StatusOK
			}
		}

		return http.StatusOK
	})

	m.Get(recvStringsPath, Messages(), func(context *macaron.Context, receiver <-chan string, done <-chan bool) int {
		for {
			select {
			case msg := <-receiver:
				recvStrings = append(recvStrings, msg)
			case <-done:
				recvStringsDone = true
				return http.StatusOK
			}
		}

		return http.StatusOK
	})

	m.Get(recvByteSlicePath, ByteSliceMessages(), func(context *macaron.Context, receiver <-chan []byte, done <-chan bool) int {
		for {
			select {
			case msg := <-receiver:
				recvByteSlices = append(recvByteSlices, msg)
			case <-done:
				recvByteSlicesDone = true
				return http.StatusOK
			}
		}

		return http.StatusOK
	})

	m.Get(sendStringsPath, Messages(), func(context *macaron.Context, sender chan<- string, done <-chan bool, disconnect chan<- int) int {
		ticker := time.NewTicker(1 * time.Millisecond)
		bomb := time.After(4 * time.Millisecond)

		for {
			select {
			case <-ticker.C:
				sender <- "Hello World"
			case <-done:
				ticker.Stop()
				sendStringsDone = true
				return http.StatusOK
			case <-bomb:
				disconnect <- websocket.CloseGoingAway

				return http.StatusOK
			}
		}

		return http.StatusOK
	})

	m.Get(sendByteSlicePath, ByteSliceMessages(), func(context *macaron.Context, sender chan<- []byte, done <-chan bool, disconnect chan<- int) int {
		ticker := time.NewTicker(1 * time.Millisecond)
		bomb := time.After(4 * time.Millisecond)

		for {
			select {
			case <-ticker.C:
				sender <- []byte("Hello World")
			case <-done:
				ticker.Stop()
				sendByteSlicesDone = true
				return http.StatusOK
			case <-bomb:
				disconnect <- websocket.CloseGoingAway

				return http.StatusOK
			}
		}

		return http.StatusOK
	})

	go m.Run()
	time.Sleep(5 * time.Millisecond)
}

func connectSocket(t *testing.T, path string) (*websocket.Conn, *http.Response) {
	header := make(http.Header)
	header.Add("Origin", host)
	ws, resp, err := websocket.DefaultDialer.Dial(endpoint+path, header)
	if err != nil {
		t.Fatalf("Connecting the socket failed: %s", err.Error())
	}

	return ws, resp
}

func TestStringReceive(t *testing.T) {
	once.Do(startServer)
	expectStringsToBeEmpty(t, recvStrings)

	ws, resp := connectSocket(t, recvStringsPath)

	ticker := time.NewTicker(time.Millisecond)

	for {
		<-ticker.C
		s := "Hello World"
		err := ws.WriteMessage(websocket.TextMessage, []byte(s))
		if err != nil {
			t.Errorf("Writing to the socket failed with %s", err.Error())
		}
		recvStringsCount++
		if recvStringsCount == 4 {
			ws.Close()
			return
		}
	}

	expectStringsToHaveArrived(t, 3, recvStrings)
	expectStatusCode(t, http.StatusSwitchingProtocols, resp.StatusCode)
	expectIsDone(t, recvStringsDone)
}

func TestStringSend(t *testing.T) {
	once.Do(startServer)
	expectStringsToBeEmpty(t, sendStrings)

	ws, resp := connectSocket(t, sendStringsPath)
	defer ws.Close()

	for {
		_, msgArray, err := ws.ReadMessage()
		msg := string(msgArray)
		sendStrings = append(sendStrings, msg)
		if sendStringsCount == 3 {
			return
		}
		if err != nil && err != io.EOF {
			t.Errorf("Receiving from the socket failed with %v", err)
		}
		sendStringsCount++
	}
	expectStringsToHaveArrived(t, 3, sendStrings)
	expectStatusCode(t, http.StatusSwitchingProtocols, resp.StatusCode)
	expectIsDone(t, sendStringsDone)
}

func TestByteSliceSend(t *testing.T) {
	once.Do(startServer)
	expectByteSlicesToBeEmpty(t, sendByteSlices)

	ws, resp := connectSocket(t, sendByteSlicePath)
	defer ws.Close()

	for {
		_, msgArray, err := ws.ReadMessage()
		sendByteSlices = append(sendByteSlices, msgArray)
		if sendStringsCount == 3 {
			return
		}
		if err != nil && err != io.EOF {
			t.Errorf("Receiving from the socket failed with %v", err)
		}
		sendStringsCount++
	}
	expectByteSlicesToHaveArrived(t, 3, sendByteSlices)
	expectStatusCode(t, http.StatusSwitchingProtocols, resp.StatusCode)
	expectIsDone(t, sendByteSlicesDone)
}

func TestJSONReceive(t *testing.T) {
	once.Do(startServer)
	expectMessagesToBeEmpty(t, recvMessages)

	ws, resp := connectSocket(t, recvPath)

	message := &Message{"Hello World"}

	ticker := time.NewTicker(time.Millisecond)

	for {
		<-ticker.C
		err := ws.WriteJSON(message)
		if err != nil {
			t.Errorf("Writing to the socket failed with %v", err)
		}
		recvCount++
		if recvCount == 4 {
			ws.Close()
			return
		}
	}

	expectMessagesToHaveArrived(t, 3, recvMessages)
	expectStatusCode(t, http.StatusSwitchingProtocols, resp.StatusCode)
	expectIsDone(t, recvDone)
}

func TestByteSliceReceive(t *testing.T) {
	once.Do(startServer)
	expectByteSlicesToBeEmpty(t, recvByteSlices)

	ws, resp := connectSocket(t, recvByteSlicePath)

	ticker := time.NewTicker(time.Millisecond)

	for {
		<-ticker.C
		s := "Hello World"
		err := ws.WriteMessage(websocket.BinaryMessage, []byte(s))
		if err != nil {
			t.Errorf("Writing to the socket failed with %s", err.Error())
		}
		recvByteSliceCount++
		if recvByteSliceCount == 4 {
			ws.Close()
			return
		}
	}

	expectByteSlicesToHaveArrived(t, 3, recvByteSlices)
	expectStatusCode(t, http.StatusSwitchingProtocols, resp.StatusCode)
	expectIsDone(t, recvByteSlicesDone)
}

func TestJSONSend(t *testing.T) {
	once.Do(startServer)
	expectMessagesToBeEmpty(t, sendMessages)

	ws, resp := connectSocket(t, sendPath)
	defer ws.Close()

	for {
		msg := &Message{}
		err := ws.ReadJSON(msg)
		sendMessages = append(sendMessages, msg)
		if sendCount == 3 {
			return
		}
		if err != nil && err != io.EOF {
			t.Errorf("Receiving from the socket failed with %v", err)
		}
		sendCount++
	}
	expectMessagesToHaveArrived(t, 3, sendMessages)
	expectStatusCode(t, http.StatusSwitchingProtocols, resp.StatusCode)
	expectIsDone(t, sendDone)
}

func TestOptionsDefaultHandling(t *testing.T) {
	o := newOptions([]*Options{
		&Options{
			LogLevel:   LogLevelDebug,
			WriteWait:  15 * time.Second,
			PingPeriod: 10 * time.Second,
		},
	})
	expectSame(t, o.LogLevel, LogLevelDebug)
	expectSame(t, o.PingPeriod, 10*time.Second)
	expectSame(t, o.WriteWait, 15*time.Second)
	expectSame(t, o.MaxMessageSize, defaultMaxMessageSize)
	expectSame(t, o.SendChannelBuffer, defaultSendChannelBuffer)
	expectSame(t, o.RecvChannelBuffer, defaultRecvChannelBuffer)
}

func TestAllowedCrossOrigin(t *testing.T) {
	m := macaron.Classic()

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/test", strings.NewReader(""))
	req.Header.Add("Origin", "http://allowed.com")

	if err != nil {
		t.Error(err)
	}

	m.Any("/test", Messages(&Options{AllowedOrigin: "https?://allowed\\.com$"}), func(context *macaron.Context, receiver <-chan string, done <-chan bool) int {
		return http.StatusOK
	})

	m.ServeHTTP(recorder, req)
	// Fails at handshake stage, so we expect 400 Bad Request
	expectStatusCode(t, http.StatusBadRequest, recorder.Code)
}

func TestDisallowedCrossOrigin(t *testing.T) {
	m := macaron.Classic()

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/test", strings.NewReader(""))
	req.Header.Add("Origin", "http://somewhere.com")

	if err != nil {
		t.Error(err)
	}

	m.Any("/test", Messages(), func() int {
		return http.StatusOK
	})

	m.ServeHTTP(recorder, req)
	expectStatusCode(t, http.StatusForbidden, recorder.Code)
}

func TestDisallowedMethods(t *testing.T) {
	m := macaron.Classic()

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest("POST", "/test", strings.NewReader(""))

	if err != nil {
		t.Error(err)
	}

	m.Any("/test", Messages(), func() int {
		return http.StatusOK
	})

	m.ServeHTTP(recorder, req)
	expectStatusCode(t, http.StatusMethodNotAllowed, recorder.Code)
}
