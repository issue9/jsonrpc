// SPDX-License-Identifier: MIT

package jsonrpc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// 定义基于流的传输层定义
type streamTransport struct {
	// header 表示是否数据流中带有报头信息
	//
	// 根据 header 的不同，初始化 buffer 或是 decoder 对象
	header  bool
	buffer  *bufio.Reader
	decoder *json.Decoder
	inMux   sync.Mutex

	out    io.Writer
	outMux sync.Mutex

	// 关闭流的函数
	close func() error
}

// 对 net.Conn 进行了自定义，使 Read 具有超时功能。
type socket struct {
	net.Conn
	timeout time.Duration
}

func (conn *socket) Read(p []byte) (int, error) {
	conn.SetReadDeadline(time.Now().Add(conn.timeout))
	return conn.Conn.Read(p)
}

func newSocketStream(conn net.Conn, timeout time.Duration) io.ReadWriteCloser {
	if timeout > 0 {
		return &socket{Conn: conn, timeout: timeout}
	}
	return conn
}

// NewSocketTransport 声明基于 net.Conn 的 Transport 实例
//
// HTTP、UDP 和 websocket 有专门的实现方法。
//
// timeout 可以使读取数据时拥有超过的功能。
// Conn.Serve() 通过 context.WithCancel 中断当前的服务，但是该功能可能由于 net.Conn.Read()
// 方法阻塞而无法真正中断服务，timeout 指定了 net.Conn.Read() 方法在无法读取数据是的超时时间。
func NewSocketTransport(header bool, conn net.Conn, timeout time.Duration) Transport {
	s := newSocketStream(conn, timeout)
	return NewStreamTransport(header, s, s, func() error { return s.Close() })
}

// NewStreamTransport 返回基于流的 Transport 实例
//
// header 是否需要解析报头内容；
// close 指定了关闭 in 和 out 的函数，如果不需要关闭，则可以传递 nil 值。
func NewStreamTransport(header bool, in io.Reader, out io.Writer, close func() error) Transport {
	t := &streamTransport{
		header: header,
		out:    out,
		close:  close,
	}

	if header {
		t.buffer = bufio.NewReader(in)
	} else {
		t.decoder = json.NewDecoder(in)
	}

	return t
}

func (s *streamTransport) Read(v interface{}) error {
	s.inMux.Lock()
	defer s.inMux.Unlock()

	if !s.header {
		return s.decoder.Decode(v)
	}

	var length int64
	for {
		line, err := s.buffer.ReadString('\n')
		if err != nil {
			return err
		}
		line = strings.TrimSpace(line)

		if line == "" { // 空行，则表示报头部分已经结束
			break
		}

		index := strings.IndexByte(line, ':')
		if index <= 0 {
			return errInvalidHeader
		}

		v := strings.TrimSpace(line[index+1:])
		switch http.CanonicalHeaderKey(strings.TrimSpace(line[:index])) {
		case contentLength:
			length, err = strconv.ParseInt(v, 10, 64)
			if err != nil {
				return err
			}
		case contentType:
			if err := validContentType(v); err != nil {
				return err
			}
		default: // 忽略其它报头
		}
	}

	switch {
	case length < 0:
		return errMissContentLength
	case length == 0:
		return nil
	}

	data := make([]byte, length)
	n, err := io.ReadFull(s.buffer, data)
	if err != nil {
		return err
	}

	return json.Unmarshal(data[:n], v)
}

var contentTypeHeader string

func init() {
	p := fmt.Sprintf("%s: %s;charset=%s\r\n%s: ", contentType, mimetypes[0], charset, contentLength)
	contentTypeHeader = p + "%d\r\n\r\n"
}

func (s *streamTransport) Write(v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}

	s.outMux.Lock()
	defer s.outMux.Unlock()

	if s.header {
		_, err = fmt.Fprintf(s.out, contentTypeHeader, len(data))
		if err != nil {
			return err
		}
	}

	_, err = s.out.Write(data)
	return err
}

func (s *streamTransport) Close() error {
	if s.close != nil {
		return s.close()
	}
	return nil
}
