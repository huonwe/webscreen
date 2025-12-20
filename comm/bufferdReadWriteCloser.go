package comm

import (
	"bufio"
	"net"
)

type BufferedReadWriteCloser struct {
	net.Conn
	br *bufio.Reader
}

func NewBufferedReadWriteCloser(conn net.Conn, size int) *BufferedReadWriteCloser {
	return &BufferedReadWriteCloser{
		Conn: conn,
		br:   bufio.NewReaderSize(conn, size),
	}
}

func (b *BufferedReadWriteCloser) Read(p []byte) (n int, err error) {
	return b.br.Read(p)
}

func (b *BufferedReadWriteCloser) Write(p []byte) (n int, err error) {
	return b.Conn.Write(p)
}

func (b *BufferedReadWriteCloser) Close() error {
	return b.Conn.Close()
}
