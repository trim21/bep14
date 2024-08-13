package bep14

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"sync"
	"syscall"

	"go.uber.org/multierr"
)

const method = "BT-SEARCH"

const btSearchLine = "BT-SEARCH * HTTP/1.1\r\n"

var hdrInfohash = http.CanonicalHeaderKey("iHash")
var hdrPort = http.CanonicalHeaderKey("port")

const host4 = "239.192.152.143:6771"
const host6 = "[ff15::efc0:988f]:6771"

var addr4 = net.UDPAddrFromAddrPort(netip.AddrPortFrom(netip.MustParseAddr("239.192.152.143"), 6771))
var addr6 = net.UDPAddrFromAddrPort(netip.AddrPortFrom(netip.MustParseAddr("ff15::efc0:988f"), 6771))

type Announce struct {
	InfoHashes []string
	Source     netip.AddrPort
}

type LSP struct {
	selfCookie string

	C <-chan Announce // exposed API

	c chan Announce

	conn4      net.PacketConn
	conn6      net.PacketConn
	clientPort string
}

func (l *LSP) Start() {
	var g sync.WaitGroup

	if l.conn4 != nil {
		l.read4()
	}

	if l.conn6 != nil {
		l.read6()
	}

	g.Wait()
}

func (l *LSP) read4() {
	var buf = make([]byte, 1400)
	for {
		n, remote, err := l.conn4.ReadFrom(buf)
		if err != nil {
			// windows may return an error if package buffer size is too small.
			// linux will just truncate the packet.

			// wsarecvfrom: Args message sent on a datagram socket was larger than the internal message buffer or some other network limit,
			//			// or the buffer used to receive a datagram into was smaller than the datagram itself.
			continue
		}

		l.handleMsg(buf[:n], remote)
	}
}

func (l *LSP) read6() {
	var buf = make([]byte, 1400)
	for {
		n, remote, err := l.conn6.ReadFrom(buf)
		if err != nil {
			// windows may return an error if package buffer size is too small.
			// linux will just truncate the packet.

			// wsarecvfrom: Args message sent on a datagram socket was larger than the internal message buffer or some other network limit,
			//			// or the buffer used to receive a datagram into was smaller than the datagram itself.
			continue
		}

		l.handleMsg(buf[:n], remote)
	}
}

func (l *LSP) handleMsg(buf []byte, remote net.Addr) {
	r, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(buf)))
	if err != nil {
		return
	}

	if r.Method != method {
		return
	}

	ih := parseList(r.Header[hdrInfohash])
	if len(ih) == 0 {
		return
	}

	for _, h := range ih {
		switch len(h) {
		case sha1.Size * 2, sha256.Size * 2:
		default:
			return
		}

		// validate hex
		for i := 0; i < len(h); i++ {
			ch := h[i]
			// 0123456789 abcdef ABCDEF
			if ('0' <= ch && ch <= '9') || ('a' <= ch && ch <= 'f') || ('A' <= ch && ch <= 'F') {
				continue
			}
		}
	}

	rawPort := r.Header.Get(hdrPort)
	if rawPort == "" {
		return
	}

	port, err := strconv.ParseUint(rawPort, 10, 16)
	if err != nil {
		return
	}

	addr, ok := netip.AddrFromSlice(remote.(*net.UDPAddr).IP)
	if !ok {
		return
	}

	source := netip.AddrPortFrom(addr, uint16(port))

	l.c <- Announce{
		InfoHashes: r.Header.Values(hdrInfohash),
		Source:     source,
	}
}

var pool = sync.Pool{New: func() any {
	return bytes.NewBuffer(make([]byte, 0, 1400))
}}

func (l *LSP) Announce(infoHashes []string) error {
	if len(infoHashes) == 0 {
		return errors.New("no infohash to announce")
	}

	return multierr.Append(l.announce4(infoHashes), l.announce6(infoHashes))
}

const sep = "\r\n"

func (l *LSP) announce4(infoHashes []string) error {
	if l.conn4 == nil {
		return nil
	}

	var b = pool.Get().(*bytes.Buffer)
	defer pool.Put(b)
	b.Reset()
	l.encodeMessage(b, host6, infoHashes)

	_, err := l.conn4.WriteTo(b.Bytes(), addr4)
	return err
}

func (l *LSP) announce6(infoHashes []string) error {
	if l.conn6 == nil {
		return nil
	}

	var b = pool.Get().(*bytes.Buffer)
	defer pool.Put(b)
	b.Reset()
	l.encodeMessage(b, host6, infoHashes)

	_, err := l.conn6.WriteTo(b.Bytes(), addr4)
	return err
}

func (l *LSP) encodeMessage(b *bytes.Buffer, host string, infoHashes []string) {
	b.WriteString(btSearchLine)

	b.WriteString("host: ")
	b.WriteString(host)
	b.WriteString(sep)

	b.WriteString("port: ")
	b.WriteString(l.clientPort)
	b.WriteString(sep)

	b.WriteString("cookie: ")
	b.WriteString(l.selfCookie)
	b.WriteString(sep)

	b.WriteString("ihash: ")
	switch len(infoHashes) {
	case 1:
		b.WriteString(infoHashes[0])
	default:
		b.WriteString(infoHashes[0])
		for _, ih := range infoHashes[1:] {
			b.WriteString(", ")
			b.WriteString(ih)
		}
	}
	b.WriteString(sep)

	b.WriteString(sep)
}

type config struct {
	enableV4 bool
	enableV6 bool
}

type option func(*config)

func EnableV4() option {
	return func(c *config) {
		c.enableV4 = true
	}
}

func EnableV6() option {
	return func(c *config) {
		c.enableV6 = true
	}
}

func New(clientPort uint16, options ...option) *LSP {
	cfg := &config{}
	for _, opt := range options {
		opt(cfg)
	}

	if !cfg.enableV4 && !cfg.enableV6 {
		panic("lsp: must enable ipv4 or ipv6")
	}

	lc := net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
			return c.Control(reusePort)
		},
	}

	c := make(chan Announce, 1)

	l := &LSP{
		clientPort: fmt.Sprintf("%d", clientPort),
		selfCookie: newCookies(12),
		C:          c,
		c:          c,
	}

	if cfg.enableV4 {
		conn, err := lc.ListenPacket(context.Background(), "udp4", addr4.String())
		if err != nil {
			panic(err)
		}
		l.conn4 = conn
	}

	if cfg.enableV6 {
		conn, err := lc.ListenPacket(context.Background(), "udp6", addr6.String())
		if err != nil {
			panic(err)
		}
		l.conn6 = conn
	}

	return l
}

// this is not very efficacy, but it's expected to be executed only once.
func newCookies(size int) string {
	// base62 chars
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"

	var b = make([]byte, 0, size)
	for i := 0; i < size; i++ {
		b = append(b, chars[rand.Intn(len(chars))])
	}

	return string(b)
}
