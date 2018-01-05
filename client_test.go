package irc

import (
	"bufio"
	"fmt"
	"net/textproto"
	"strings"
	"sync"
	"testing"
)

// clientTest contains the structure of the test cases
type clientTest struct {
	name    string
	script  []string
	events  []string
	handler func(m *Message)
}

// clientTests holds all the test cases
var clientTests = []clientTest{

	{
		name:   "nick in use",
		events: []string{"433"},
		script: []string{
			"CLI USER bar * * :foo bar",
			"CLI NICK foo",
			"SRV :irc.example.net 433 * foo :Nickname already in use",
			"CLI NICK foo_",
			"SRV :irc.example.net 433 * foo_ :Nickname already in use",
			"CLI NICK foo__",
			"SRV :irc.example.net 433 * foo__ :Nickname already in use",
			"CLI NICK foo___",
			"SRV ERROR :end of test",
		},
	},
	{
		name:   "ping pong",
		events: []string{"PING"},
		script: []string{
			"CLI USER bar * * :foo bar",
			"CLI NICK foo",
			"SRV PING :irc.example.net",
			"CLI PONG :irc.example.net",
			"SRV ERROR :end of test",
		},
	},
	{
		name:   "ctcp version",
		events: []string{"PRIVMSG"},
		script: []string{
			"CLI USER bar * * :foo bar",
			"CLI NICK foo",
			"SRV :bar!bar@127.0.0.1 PRIVMSG foo :\x01VERSION\x01",
			"CLI NOTICE bar :\x01VERSION the irc lib\x01",
			"SRV ERROR :end of test",
		},
	},
	{
		name:   "reclaim nick",
		events: []string{"433", "PING", "401"},
		script: []string{
			"CLI USER bar * * :foo bar",
			"CLI NICK foo",
			"SRV :irc.example.net 433 * foo :Nickname already in use",
			"CLI NICK foo_",
			"SRV PING :irc.example.net",
			"CLI PONG :irc.example.net",
			"CLI WHOIS foo",
			"SRV :irc.example.net 401 foo_ foo :No such nick or channel name",
			"CLI NICK foo",
			"SRV ERROR :end of test",
		},
	},
}

// TestClient tests all client test cases
func TestClient(t *testing.T) {
	// Iterate over all test cases
	for _, ct := range clientTests {
		t.Run(ct.name, func(t *testing.T) {
			testClient(&ct, t)
		})
	}
}

// testClient takes a client test and executes it
func testClient(ct *clientTest, t *testing.T) {
	// Extract the client and server scripts from the main script
	var clientScript []string
	var serverScript []string

	for _, s := range ct.script {
		if strings.Index(s, "SRV") == 0 {
			serverScript = append(serverScript, s)
		} else if strings.Index(s, "CLI") == 0 {
			clientScript = append(clientScript, s)
		}
	}

	// Create a new mocked communications hub
	// The hub contains a server to client and client to server pipe
	// This is needed so we can simulate data sent both from the client to server
	// and from the server back to the client
	conn := newMockComm()

	// Wait group that keeps track of the IRC client and the event handlers
	// Each SRV line in a script is handled by an event handler
	// Since event handlers are async we need to add them to the wait group
	var wg sync.WaitGroup
	wg.Add(1 + len(serverScript))

	// Create a new IRC client with our mocked connection
	c := NewClient(
		WithConn(conn.Client),
		WithNick("foo"),
		WithUser("bar"),
		WithRealName("foo bar"),
		WithVersion("the irc lib"))

	// Connect to the IRC server
	// Since the Connect call blocks we need to run this in a goroutine
	go func() {
		c.Connect()
		wg.Done()
	}()

	// Mutex and map of all messages that are received by the event handlers
	// Since they are run async they can be accessed in an unpredictable order
	// So we make the comparisson of the expected results after the script has been fully executed
	var mu sync.Mutex
	hm := make(map[string]bool)

	// Register the event handled that is defined by the test
	for _, e := range ct.events {
		c.Handle(e, func(m *Message) {
			// Store the message in the map
			mu.Lock()
			hm[m.Raw] = true
			mu.Unlock()

			// Notify the wait group that one of the events has been handled
			wg.Done()
		})
	}

	// We use this event handler to signal to our client that the test connection should be closed
	c.Handle("ERROR", func(m *Message) {
		// Close the client and server pipes
		conn.Client.Close()
		conn.Server.Close()

		// Record the message in the map so we can compare it
		mu.Lock()
		hm[m.Raw] = true
		mu.Unlock()

		// Tell the wait group that we are done
		wg.Done()
	})

	// Create reader for the server connection
	rd := bufio.NewReader(conn.Server)
	tr := textproto.NewReader(rd)

	// Iterate over the script
	for _, script := range ct.script {
		// Extract the script type
		typ := script[0:3]

		// Message
		s := script[4:]

		// CLI indicates that we expect a message to be sent from the client to the server
		// So we wait until a message has been read and verifies it against the script
		if typ == "CLI" {
			l, _ := tr.ReadLine()

			if l != s {
				t.Errorf("%s: client sent unexpected data to the server", ct.name)
				t.Logf("sent: %s", l)
				t.Logf("expected: %s", s)
			}
		}

		// SRV means that the client should receive data from the server
		// So we write the message on the server connection and wait for our event handler
		// to pick up the message and handle it
		if typ == "SRV" {
			fmt.Fprintf(conn.Server, s+eol)
		}
	}

	// Wait until everything has been executet
	wg.Wait()

	// Iterate over the script again and find all SRV entries
	// There should be an equivalent message in the hm map for each SRV entry
	// If it doesn't match something is wrong
	for _, script := range serverScript {
		// Extract the expected message
		s := script[4:]

		// Make sure that the map contains the message
		if _, ok := hm[s]; !ok {
			t.Errorf("%s: client should have receieved %s from the server", ct.name, s)
		}
	}
}
