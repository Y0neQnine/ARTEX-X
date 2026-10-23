package webshell

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Autumn-27/artex/db"
)

// fakeShell 起一个本地 HTTP 服务模拟菜刀式 webshell：cmd 参数原样回显。
func fakeShell(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/shell.php", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		cmd := r.URL.Query().Get("cmd")
		if cmd == "" {
			cmd = r.FormValue("cmd")
		}
		_, _ = w.Write([]byte("echo: " + cmd))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestExecPost(t *testing.T) {
	srv := fakeShell(t)
	ws := db.Webshell{URL: srv.URL + "/shell.php", Method: "POST", ParamName: "cmd"}
	res := Exec(context.Background(), ws, "whoami", 5*time.Second)
	if res.Status != "ok" {
		t.Fatalf("status = %s, output = %q", res.Status, res.Output)
	}
	if res.Output != "echo: whoami" {
		t.Fatalf("unexpected output: %q", res.Output)
	}
	if res.DurationMS < 0 {
		t.Fatalf("negative duration: %d", res.DurationMS)
	}
}

func TestExecGet(t *testing.T) {
	srv := fakeShell(t)
	ws := db.Webshell{URL: srv.URL + "/shell.php?x=1", Method: "GET", ParamName: "cmd"}
	res := Exec(context.Background(), ws, "id", 5*time.Second)
	if res.Status != "ok" || res.Output != "echo: id" {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestExecHeaders(t *testing.T) {
	srv := fakeShell(t)
	ws := db.Webshell{
		URL: srv.URL + "/shell.php", Method: "POST", ParamName: "cmd",
		Headers: map[string]string{"X-Token": "abc123", "User-Agent": "custom-ua"},
	}
	// 服务端 echo 不校验头；这里只验证请求不因头而出错（无 panic / 连接正常）。
	res := Exec(context.Background(), ws, "pwd", 5*time.Second)
	if res.Status != "ok" {
		t.Fatalf("status = %s, output = %q", res.Status, res.Output)
	}
}

func TestExecHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "denied", http.StatusForbidden)
	}))
	defer srv.Close()
	ws := db.Webshell{URL: srv.URL, Method: "POST", ParamName: "cmd"}
	res := Exec(context.Background(), ws, "id", 5*time.Second)
	if res.Status != "error" {
		t.Fatalf("expected error status, got %s", res.Status)
	}
}

func TestExecTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
	}))
	defer srv.Close()
	ws := db.Webshell{URL: srv.URL, Method: "POST", ParamName: "cmd"}
	start := time.Now()
	res := Exec(context.Background(), ws, "sleep 10", 200*time.Millisecond)
	if res.Status != "error" {
		t.Fatalf("expected error status, got %s: %q", res.Status, res.Output)
	}
	if time.Since(start) > 1500*time.Millisecond {
		t.Fatalf("timeout not honored: %v", time.Since(start))
	}
}

func TestDecodeBody(t *testing.T) {
	if s := decodeBody([]byte{0xB2, 0xE2, 0xCA, 0xD4}, "gbk"); s != "测试" {
		t.Fatalf("gbk decode = %q", s)
	}
	if s := decodeBody([]byte("plain"), ""); s != "plain" {
		t.Fatalf("utf8 passthrough = %q", s)
	}
}

func TestValidate(t *testing.T) {
	cases := []struct {
		name, url, method, param string
		wantErr                  bool
	}{
		{"", "http://a/s.php", "POST", "cmd", true},
		{"s", "ftp://a/s.php", "POST", "cmd", true},
		{"s", "http://a/s.php", "PUT", "cmd", true},
		{"s", "http://a/s.php", "POST", "", true},
		{"s", "http://a/s.php", "", "cmd", true},
		{"s", "http://a/s.php", "POST", "cmd", false},
	}
	for i, c := range cases {
		err := Validate(c.name, c.url, c.method, c.param)
		if (err != nil) != c.wantErr {
			t.Fatalf("case %d (%s %s %s %s): err = %v, wantErr = %v", i, c.name, c.url, c.method, c.param, err, c.wantErr)
		}
	}
}
