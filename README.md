jsonrpc
[![Build Status](https://img.shields.io/endpoint.svg?url=https%3A%2F%2Factions-badge.atrox.dev%2Fissue9%2Fjsonrpc%2Fbadge%3Fref%3Dmaster&style=flat)](https://actions-badge.atrox.dev/issue9/jsonrpc/goto?ref=master)
[![codecov](https://codecov.io/gh/issue9/jsonrpc/branch/master/graph/badge.svg)](https://codecov.io/gh/issue9/jsonrpc)
[![license](https://img.shields.io/badge/license-MIT-brightgreen.svg?style=flat)](https://opensource.org/licenses/MIT)
======

JSON RPC 2.0 的实现，目前实现了对以下传输层的接口：

- socket, net 包中所有支持 Conn 接口的实现；
- websocket, 采用了 github.com/gorilla/websocket 作为底层调用；
- HTTP 普通的 HTTP 请求方式；

*目前不支持批处理模式*

### Socket

```go
srv := NewServer()
listen, err := net.Listen("tcp", ":8080")
for {
    c, err := listen.Accept()
    conn := srv.NewConn(NewSocketTransport(true, c), nil)
    conn.Serve(ctx)

    // 主动请求客户端
    conn.Send("/method", in, func(result *result) error {
        // 此处用于处理返回的数据
    })
}
```

### HTTP

```go
srv := NewServer()
conn := srv.NewHTTPConn(nil)
http.Handle(conn)
```

安装
----

```shell
go get github.com/issue9/jsonrpc
```

文档
----

[![Go Walker](https://gowalker.org/api/v1/badge)](https://gowalker.org/github.com/issue9/jsonrpc)
[![PkgGoDev](https://pkg.go.dev/badge/github.com/issue9/jsonrpc)](https://pkg.go.dev/github.com/issue9/jsonrpc)

版权
----

本项目采用 [MIT](https://opensource.org/licenses/MIT) 开源授权许可证，完整的授权说明可在 [LICENSE](LICENSE) 文件中找到。
