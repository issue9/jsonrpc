// SPDX-License-Identifier: MIT

package jsonrpc

import (
    "bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

var (
	ErrInvalidHeader = errors.New("无效的报头")
)

// content-type json-rpc 采用的字符集
const charset = "utf-8"

var (
	contentType   = http.CanonicalHeaderKey("content-Type")
	contentLength = http.CanonicalHeaderKey("content-length")
)

type httpTransport struct {
	in     io.Reader
	out    io.Writer
	outMux sync.Mutex
}

// NewHTTPTransport 声明基于 HTTP 的 transport 实例
func NewHTTPTransport(in io.Reader, out io.Writer) Transport {
	return &httpTransport{
		in:  in,
		out: out,
	}
}

// Read 读取内容，先验证报头，并返回实际 body 的内容
func (s *httpTransport) Read(v interface{}) error {
	buf := bufio.NewReader(s.in)
	var l int

	for {
		line, err := buf.ReadString('\n')
		if err != nil {
			return err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}

		index := strings.IndexByte(line, ':')
		if index <= 0 {
			return ErrInvalidHeader
		}

		v := strings.TrimSpace(line[index+1:])
		switch http.CanonicalHeaderKey(strings.TrimSpace(line[:index])) {
		case contentLength:
			l, err = strconv.Atoi(v)
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

	if l <= 0 {
		return locale.Errorf(locale.ErrInvalidContentLength)
	}

	data := make([]byte, l)
	n, err := io.ReadFull(buf, data)
	if err != nil {
		return err
	}
	if n == 0 {
		return locale.Errorf(locale.ErrBodyIsEmpty)
	}

	return json.Unmarshal(data[:n], v)
}

func validContentType(header string) error {
	pairs := strings.Split(header, ";")

	for _, pair := range pairs {
		index := strings.IndexByte(pair, '=')
		if index > 0 &&
			strings.ToLower(strings.TrimSpace(pair[:index])) == "charset" &&
			strings.ToLower(strings.TrimSpace(pair[index+1:])) != charset {
			return locale.Errorf(locale.ErrInvalidContentTypeCharset)
		}
	}

	return nil
}

func (s *httpTransport) Write(obj interface{}) error {
	data, err := json.Marshal(obj)
	if err != nil {
		return err
	}

	s.outMux.Lock()
	defer s.outMux.Unlock()

	_, err = fmt.Fprintf(s.out, "%s: %s\r\n%s: %d\r\n\r\n", contentType, charset, contentLength, len(data))
	if err != nil {
		return err
	}

	_, err = s.out.Write(data)
	return err
}
