package irc

import (
	"reflect"
	"testing"
)

// messageTest defines the structure for a test case
type messageTest struct {
	name string
	raw  string
	msg  *Message
	err  bool
}

// messageTests defines all test cases
var messageTests = []messageTest{
	{
		name: "join",
		raw:  ":foo!~bar@127.0.0.1 JOIN :#foo\r\n",
		msg: &Message{
			Command:     "JOIN",
			Params:      ":#foo",
			ParamsArray: []string{":#foo"},
			Name:        "foo",
			User:        "~bar",
			Host:        "127.0.0.1",
		},
	},
	{
		name: "motd",
		raw:  ":irc.foo.com 372 foo :- *  foo\r\n",
		msg: &Message{
			Command:     "372",
			Params:      "foo :- * foo",
			ParamsArray: []string{"foo", ":-", "*", "foo"},
			Name:        "irc.foo.com",
		},
	},
	{
		name: "ping",
		raw:  "PING :irc.foo.com\r\n",
		msg: &Message{
			Command:     "PING",
			Params:      ":irc.foo.com",
			ParamsArray: []string{":irc.foo.com"},
		},
	},
	{
		name: "malformed",
		raw:  "foo:\r\n",
		err:  true,
	},
	{
		name: "empty",
		raw:  "\r\n",
	},
}

// Run all tests
func TestMessages(t *testing.T) {
	for _, mt := range messageTests {
		t.Run(mt.name, func(t *testing.T) {
			// Parse the raw message
			m, err := parse(mt.raw)

			// Make sure the error is set if the test expects it to be
			if mt.err && err == nil {
				t.Errorf("%s: should return an error but didn't", mt.name)
			}

			// If not the msg is nil, assign the raw value to it
			if mt.msg != nil {
				mt.msg.Raw = mt.raw
			}

			// Compare the parsed message with what we expect it to be
			if !reflect.DeepEqual(mt.msg, m) {
				t.Errorf("%s: failed to parse message %s", mt.name, mt.raw)
				t.Logf("output: %#v", m)
				t.Logf("expected: %#v", mt.msg)
			}
		})
	}
}
