// SPDX-License-Identifier: MIT

package jsonrpc

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

// 一些错误定义
var (
	ErrInvalidHeader      = errors.New("无效的报头格式")
	ErrInvalidContentType = errors.New("无效的报头 Content-Type")
	ErrMissContentLength  = errors.New("缺少 Content-Length 报头")
)

var (
	contentType   = http.CanonicalHeaderKey("content-Type")
	contentLength = http.CanonicalHeaderKey("content-length")
)

// 可能的 mimetype 值，第一个元素作为默认值，在输出时使用。
//
// https://www.jsonrpc.org/historical/json-rpc-over-http.html#id13
var mimetypes = []string{
	"application/json",
	"application/json-rpc",
	"application/jsonrequest",
}

const charset = "utf-8"

// HTTPConn 表示 json rpc 的 HTTP 服务端中间件
type HTTPConn struct {
	server *Server
	errlog *log.Logger
	url    string
}

type httpTransport struct {
	r    *http.Request
	w    http.ResponseWriter
	wMux sync.Mutex
}

// NewHTTPConn 声明 HTTP 服务端中间件
//
// url 表示主动请求时的 URL 地址，如果不需要，可以传递空值；
// errlog 表示错误日志输出通道，不需要可以为空。
func (s *Server) NewHTTPConn(url string, errlog *log.Logger) *HTTPConn {
	return &HTTPConn{
		server: s,
		errlog: errlog,
		url:    url,
	}
}

func (h *HTTPConn) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	t := newHTTPTransport(w, r)

	f, err := h.server.read(t)
	if err != nil && h.errlog != nil {
		h.errlog.Println(err)
	}

	if f == nil {
		panic("f 不能为空值")
	}

	if err = f(); err != nil {
		h.errlog.Println(err)
	}
}

// Notify 请求 JSON RPC 服务端
func (h *HTTPConn) Notify(method string, params interface{}) error {
	return h.request(method, true, params, nil)
}

// Send 请求 JSON RPC 服务端
func (h *HTTPConn) Send(method string, params, result interface{}) error {
	return h.request(method, false, params, result)
}

func (h *HTTPConn) request(method string, notify bool, in, out interface{}) error {
	var params *json.RawMessage
	if in != nil {
		data, err := json.Marshal(in)
		if err != nil {
			return err
		}
		params = (*json.RawMessage)(&data)
	}

	req := &request{
		Version: Version,
		Method:  method,
		Params:  params,
	}
	if !notify {
		req.ID = h.server.id()
	}

	body, err := json.Marshal(req)
	if err != nil {
		return err
	}

	resp, err := http.Post(h.url, mimetypes[0], bytes.NewBuffer(body))
	if err != nil {
		return err
	}

	if notify {
		return nil
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	defer func() {
		if err1 := resp.Body.Close(); err != nil {
			err = fmt.Errorf("在抛出错误 %s 时再次发生错误 %w", err.Error(), err1)
		}
	}()

	r := &response{}
	if err = json.Unmarshal(data, r); err != nil {
		return err
	}

	if r.ID != nil && !req.ID.Equal(r.ID) {
		return NewError(CodeInvalidParams, "id not equal")
	}

	if r.Error != nil {
		return r.Error
	}

	if r.Result == nil {
		return nil
	}
	return json.Unmarshal(*r.Result, out)
}

// 声明基于 HTTP 的 Transport 实例
func newHTTPTransport(w http.ResponseWriter, r *http.Request) Transport {
	return &httpTransport{
		r: r,
		w: w,
	}
}

func (s *httpTransport) Read(v interface{}) error {
	if err := validContentType(s.r.Header.Get(contentType)); err != nil {
		return err
	}

	cl := s.r.Header.Get(contentLength)
	if cl == "" {
		return ErrMissContentLength
	}
	l, err := strconv.ParseInt(cl, 10, 64)
	if err != nil {
		return err
	}

	data := make([]byte, l)
	n, err := io.ReadFull(s.r.Body, data)
	if err != nil {
		return err
	}

	return json.Unmarshal(data[:n], v)
}

func (s *httpTransport) Write(obj interface{}) error {
	data, err := json.Marshal(obj)
	if err != nil {
		return err
	}

	s.wMux.Lock()
	defer s.wMux.Unlock()

	s.w.Header().Set(contentType, mimetypes[0])
	s.w.Header().Set(contentLength, strconv.Itoa(len(data)))
	_, err = s.w.Write(data)
	return err
}

func (s *httpTransport) Close() error {
	return nil
}

// 验证 content-type 的正确性
//
// 如果存在该值，则必须要以 mimetype 开头，
// charset 如果有指定，必须为 utf-8，否则不作判断
func validContentType(header string) error {
	if header == "" {
		return nil
	}

	pairs := strings.Split(header, ";")

	var found bool
	mimetype := strings.ToLower(pairs[0])
	for _, item := range mimetypes {
		if mimetype == item {
			found = true
			break
		}
	}
	if !found {
		return ErrInvalidContentType
	}

	for _, pair := range pairs[1:] {
		index := strings.IndexByte(pair, '=')
		if index > 0 &&
			strings.ToLower(strings.TrimSpace(pair[:index])) == "charset" &&
			strings.ToLower(strings.TrimSpace(pair[index+1:])) != charset {
			return ErrInvalidContentType
		}
	}

	return nil
}
