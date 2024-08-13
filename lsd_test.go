package bep14

import (
	"bufio"
	"bytes"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWriteHeader(t *testing.T) {
	var b = bytes.NewBuffer(nil)

	l := LSP{
		clientPort: "100",
		selfCookie: "0123456",
	}
	l.encodeMessage(b, "host4", []string{"h1", "h2"})

	req, err := http.ReadRequest(bufio.NewReader(b))
	if err != nil {
		panic(err)
	}

	require.Equal(t, "BT-SEARCH", req.Method)
	require.Equal(t, "host4", req.Host)
	require.Equal(t, l.selfCookie, req.Header.Get("Cookie"))
}
