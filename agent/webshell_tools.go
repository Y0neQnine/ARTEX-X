package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Autumn-27/artex/db"
	"github.com/Autumn-27/artex/webshell"
	actool "github.com/Autumn-27/norma/tool"
)

// SetWebshellStore wires the webshell store onto this ToolSet so the
// register_webshell / list_webshells / webshell_exec tools are active.
func (t *ToolSet) SetWebshellStore(w *db.WebshellStore) { t.ws = w }

// registerWebshell 让 agent 把在目标上拿到的 shell 登记进系统——前端「Webshell」
// 页随即可以查看并交互连接。task 归属由程序经 SetTaskID 权威赋值，不接受模型传入。
func (t *ToolSet) registerWebshell() actool.CoreTool {
	return writeTool(
		"register_webshell",
		"把在目标上拿到的 shell（上传的 webshell 木马、漏洞命令执行点等 HTTP 命令通道）登记进平台，前端「Webshell」页即可查看并连接交互。\n"+
			"url 是执行命令的地址；method 为请求方式（POST 表单 / GET URL 参数）；param_name 是承载命令的参数名（默认 cmd）。\n"+
			"headers 可带 Cookie / Authorization 等鉴权信息（键值对对象）。目标输出乱码时给 encoding：gbk / gb18030 / big5。\n"+
			"返回登记后的 {id}，后续可用 webshell_exec 通过该 shell 在目标上执行命令。",
		obj(map[string]any{
			"name":       str("给这个 shell 起个名，如「后台 admin.php 菜刀」「某接口 RCE」"),
			"url":        str("执行命令的完整 URL，如 http://target/shell.php"),
			"method":     str("请求方式：POST(表单参数) 或 GET(URL 参数)，默认 POST"),
			"param_name": str("承载命令的参数名，默认 cmd"),
			"headers": map[string]any{
				"type":        "object",
				"description": "附加请求头键值对，如 {\"Cookie\": \"PHPSESSID=...\", \"Authorization\": \"Basic ...\"}",
			},
			"encoding": str("输出编码：留空(utf-8) / gbk / gb18030 / big5"),
			"note":     str("备注：如何拿到、密码、路径、目标信息等"),
		}, "name", "url"),
		func(_ context.Context, in json.RawMessage) (actool.Result, error) {
			if t.ws == nil {
				return actool.Errorf("register_webshell 未启用: WebshellStore 未初始化"), nil
			}
			var a struct {
				Name      string            `json:"name"`
				URL       string            `json:"url"`
				Method    string            `json:"method"`
				ParamName string            `json:"param_name"`
				Headers   map[string]string `json:"headers"`
				Encoding  string            `json:"encoding"`
				Note      string            `json:"note"`
			}
			if err := json.Unmarshal(in, &a); err != nil {
				return actool.Errorf("invalid input: " + err.Error()), nil
			}
			method := strings.ToUpper(strings.TrimSpace(a.Method))
			if method == "" {
				method = "POST"
			}
			param := strings.TrimSpace(a.ParamName)
			if param == "" {
				param = "cmd"
			}
			if err := webshell.Validate(a.Name, a.URL, method, param); err != nil {
				return actool.Errorf(err.Error()), nil
			}
			var taskID *int64
			if t.taskID > 0 {
				taskID = &t.taskID
			}
			w, err := t.ws.Create(a.Name, strings.TrimSpace(a.URL), method, param, a.Headers,
				strings.ToLower(strings.TrimSpace(a.Encoding)), strings.TrimSpace(a.Note), taskID, nil)
			if err != nil {
				return actool.Errorf("登记失败: " + err.Error()), nil
			}
			return jsonResult(map[string]any{
				"id": w.ID, "name": w.Name, "url": w.URL, "method": w.Method, "param_name": w.ParamName,
			})
		},
	)
}

// listWebshells lets an agent enumerate the shells registered on the platform.
func (t *ToolSet) listWebshells() actool.CoreTool {
	return readTool(
		"list_webshells",
		"列出平台已登记的 webshell（agent 注册的 + 用户手工添加的）：{id, name, url, method, param_name, note, task_id}。执行命令用 webshell_exec。",
		obj(map[string]any{}),
		func(_ context.Context, _ json.RawMessage) (actool.Result, error) {
			if t.ws == nil {
				return actool.Errorf("list_webshells 未启用: WebshellStore 未初始化"), nil
			}
			items, err := t.ws.List()
			if err != nil {
				return actool.Errorf("查询失败: " + err.Error()), nil
			}
			out := make([]map[string]any, 0, len(items))
			for _, w := range items {
				out = append(out, map[string]any{
					"id": w.ID, "name": w.Name, "url": w.URL, "method": w.Method,
					"param_name": w.ParamName, "note": w.Note, "task_id": w.TaskID,
					"last_exec_at": w.LastExecAt,
				})
			}
			return jsonResult(map[string]any{"count": len(out), "webshells": out})
		},
	)
}

// webshellExec runs one command on a registered webshell and records the history.
func (t *ToolSet) webshellExec() actool.CoreTool {
	return writeTool(
		"webshell_exec",
		"通过已登记的 webshell（见 register_webshell / list_webshells）在目标上执行一条命令并取回输出。\n"+
			"执行结果同时落库，前端「Webshell」页可见完整历史。输出编码乱码时可在该 shell 的登记信息里改 encoding。",
		obj(map[string]any{
			"id":      intp("webshell 登记 id（register_webshell 返回 / list_webshells 查询）"),
			"command": str("要在目标上执行的命令，如 id;whoami;ipconfig"),
			"timeout": intp("超时秒数（默认 30，最大 120）"),
		}, "id", "command"),
		func(ctx context.Context, in json.RawMessage) (actool.Result, error) {
			if t.ws == nil {
				return actool.Errorf("webshell_exec 未启用: WebshellStore 未初始化"), nil
			}
			var a struct {
				ID      int64  `json:"id"`
				Command string `json:"command"`
				Timeout int    `json:"timeout"`
			}
			if err := json.Unmarshal(in, &a); err != nil {
				return actool.Errorf("invalid input: " + err.Error()), nil
			}
			if a.ID <= 0 {
				return actool.Errorf("id 无效"), nil
			}
			if strings.TrimSpace(a.Command) == "" {
				return actool.Errorf("command 不能为空"), nil
			}
			w, err := t.ws.Get(a.ID)
			if err != nil {
				return actool.Errorf("查询 webshell 失败: " + err.Error()), nil
			}
			if w == nil {
				return actool.Errorf(fmt.Sprintf("webshell %d 不存在（可能已被删除），请用 list_webshells 查最新列表", a.ID)), nil
			}
			res := webshell.Exec(ctx, *w, a.Command, time.Duration(a.Timeout)*time.Second)
			if _, err := t.ws.RecordExec(w.ID, a.Command, res.Output, res.Status, int(res.DurationMS)); err != nil {
				// 执行结果仍返回，只影响历史记录
				return jsonResult(map[string]any{"result": res, "history_error": err.Error()})
			}
			return jsonResult(res)
		},
	)
}
