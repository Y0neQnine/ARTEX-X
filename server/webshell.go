package server

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Autumn-27/artex/db"
	"github.com/Autumn-27/artex/webshell"
)

// webshellReq 是创建/更新 webshell 的请求体（agent 工具与 UI 共用同一校验）。
type webshellReq struct {
	Name      string            `json:"name"`
	URL       string            `json:"url"`
	Method    string            `json:"method"`
	ParamName string            `json:"param_name"`
	Headers   map[string]string `json:"headers"`
	Encoding  string            `json:"encoding"`
	Note      string            `json:"note"`
	TaskID    *int64            `json:"task_id"`
	AssetID   *int64            `json:"asset_id"`
}

func normalizeWebshellReq(req *webshellReq) {
	req.Method = strings.ToUpper(strings.TrimSpace(req.Method))
	if req.Method == "" {
		req.Method = "POST"
	}
	req.ParamName = strings.TrimSpace(req.ParamName)
	if req.ParamName == "" {
		req.ParamName = "cmd"
	}
	req.URL = strings.TrimSpace(req.URL)
	req.Encoding = strings.ToLower(strings.TrimSpace(req.Encoding))
	req.Note = strings.TrimSpace(req.Note)
}

func (s *Server) webshellList(w http.ResponseWriter, r *http.Request) {
	pg := s.pg(w)
	if pg == nil {
		return
	}
	items, err := pg.Webshells().List()
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if items == nil {
		items = []db.Webshell{}
	}
	writeJSON(w, 200, map[string]any{"webshells": items})
}

func (s *Server) webshellCreate(w http.ResponseWriter, r *http.Request) {
	pg := s.pg(w)
	if pg == nil {
		return
	}
	var req webshellReq
	if err := decode(r, &req); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	normalizeWebshellReq(&req)
	if err := webshell.Validate(req.Name, req.URL, req.Method, req.ParamName); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	item, err := pg.Webshells().Create(req.Name, req.URL, req.Method, req.ParamName, req.Headers, req.Encoding, req.Note, req.TaskID, req.AssetID)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, item)
}

func (s *Server) webshellUpdate(w http.ResponseWriter, r *http.Request) {
	pg := s.pg(w)
	if pg == nil {
		return
	}
	id, ok := pathInt(r, "id")
	if !ok {
		writeErr(w, 400, "bad webshell id")
		return
	}
	var req webshellReq
	if err := decode(r, &req); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	normalizeWebshellReq(&req)
	if err := webshell.Validate(req.Name, req.URL, req.Method, req.ParamName); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	item, err := pg.Webshells().Update(id, req.Name, req.URL, req.Method, req.ParamName, req.Headers, req.Encoding, req.Note, req.TaskID, req.AssetID)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, item)
}

func (s *Server) webshellDelete(w http.ResponseWriter, r *http.Request) {
	pg := s.pg(w)
	if pg == nil {
		return
	}
	id, ok := pathInt(r, "id")
	if !ok {
		writeErr(w, 400, "bad webshell id")
		return
	}
	if err := pg.Webshells().Delete(id); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) webshellExec(w http.ResponseWriter, r *http.Request) {
	pg := s.pg(w)
	if pg == nil {
		return
	}
	id, ok := pathInt(r, "id")
	if !ok {
		writeErr(w, 400, "bad webshell id")
		return
	}
	var req struct {
		Command string `json:"command"`
		Timeout int    `json:"timeout"`
	}
	if err := decode(r, &req); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if strings.TrimSpace(req.Command) == "" {
		writeErr(w, 400, "command 不能为空")
		return
	}
	item, err := pg.Webshells().Get(id)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if item == nil {
		writeErr(w, 404, "webshell 不存在")
		return
	}
	res := webshell.Exec(r.Context(), *item, req.Command, time.Duration(req.Timeout)*time.Second)
	if _, err := pg.Webshells().RecordExec(id, req.Command, res.Output, res.Status, int(res.DurationMS)); err != nil {
		// 结果仍返回，仅历史记录失败
		writeJSON(w, 200, map[string]any{"result": res, "history_error": err.Error()})
		return
	}
	writeJSON(w, 200, res)
}

func (s *Server) webshellCommands(w http.ResponseWriter, r *http.Request) {
	pg := s.pg(w)
	if pg == nil {
		return
	}
	id, ok := pathInt(r, "id")
	if !ok {
		writeErr(w, 400, "bad webshell id")
		return
	}
	limit := 200
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	items, err := pg.Webshells().ListCommands(id, limit)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if items == nil {
		items = []db.WebshellCommand{}
	}
	writeJSON(w, 200, map[string]any{"commands": items})
}
