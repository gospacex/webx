package webx

// testhelpers_test.go 两测试文件共享的测试辅助（hertz 客户端 / protocol 构造）。

import (
	"testing"

	"github.com/cloudwego/hertz/pkg/app/client"
	"github.com/cloudwego/hertz/pkg/protocol"
)

// newHertzClient 测试内 hertz 客户端（默认 HTTP）
func newHertzClient(t *testing.T) *client.Client {
	t.Helper()
	hc, err := client.NewClient()
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return hc
}

// newProtocolRequest 构造 protocol 请求（https 由 URL 前缀自动识别，v0.10.6 client 检测 scheme）
func newProtocolRequest(t *testing.T, method, url string) *protocol.Request {
	t.Helper()
	req := &protocol.Request{}
	req.SetMethod(method)
	req.SetRequestURI(url)
	return req
}

// newProtocolResponse 空 protocol 响应
func newProtocolResponse() *protocol.Response { return &protocol.Response{} }
