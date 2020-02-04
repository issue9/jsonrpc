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

	"github.com/issue9/autoinc"
)

// 一些错误定义
var (
	ErrInvalidContentType = errors.New("无效的报头 Content-Type")
	ErrMissContentLength  = errors.New("缺少 Content-Length 报头")
)

const (
	contentType   = "content-Type"
	contentLength = "content-length"
	charset       = "utf-8"
	mimetype      = "application/json"
)

// HTTPServer 表示 json rpc 的 HTTP 服务端中间件
type HTTPServer struct {
	server  *Server
	errlog  *log.Logger
	autoinc *autoinc.AutoInc
}

// HTTPClient http 的客户端
type HTTPClient struct {
	autoinc *autoinc.AutoInc
	url     string
}

type httpTransport struct {
	r    *http.Request
	w    http.ResponseWriter
	wMux sync.Mutex
}

// NewHTTPClient 声明新的 HTTPClient 对象
func NewHTTPClient(url string) *HTTPClient {
	return &HTTPClient{
		autoinc: autoinc.New(0, 1, 100),
		url:     url,
	}
}

// NewHTTPServer 声明 HTTP 服务端中间件
func (s *Server) NewHTTPServer(errlog *log.Logger) *HTTPServer {
	return &HTTPServer{
		server:  s,
		errlog:  errlog,
		autoinc: autoinc.New(0, 1, 100),
	}
}

func (http *HTTPServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	t := newHTTPTransport(w, r)
	if err := http.server.serve(t); err != nil && http.errlog != nil {
		http.errlog.Println(err)
	}
}

// Notify 请求 JSON RPC 服务端
func (client *HTTPClient) Notify(method string, params interface{}) error {
	return client.request(method, true, params, nil)
}

// Send 请求 JSON RPC 服务端
func (client *HTTPClient) Send(method string, params, result interface{}) error {
	return client.request(method, false, params, result)
}

func (client *HTTPClient) request(method string, notify bool, params, result interface{}) error {
	data, err := json.Marshal(params)
	if err != nil {
		return err
	}

	req := &request{
		Version: Version,
		Method:  method,
		Params:  (*json.RawMessage)(&data),
	}
	if !notify {
		req.ID = &requestID{isNumber: true, number: client.autoinc.MustID()}
	}

	body, err := json.Marshal(req)
	if err != nil {
		return err
	}

	resp, err := http.Post(client.url, mimetype, bytes.NewBuffer(body))
	if err != nil {
		return err
	}

	if notify {
		return nil
	}

	data, err = ioutil.ReadAll(resp.Body)
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

	if r.Error != nil {
		return r.Error
	}
	return json.Unmarshal(*r.Result, result)
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

	s.w.Header().Set(contentType, mimetype)
	s.w.Header().Set(contentLength, strconv.Itoa(len(data)))
	_, err = s.w.Write(data)
	return err
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

	if strings.ToLower(pairs[0]) != mimetype {
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
