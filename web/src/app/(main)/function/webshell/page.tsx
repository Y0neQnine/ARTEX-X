"use client";

import * as React from "react";

import { CircleIcon, EraserIcon, Loader2Icon, PencilIcon, PlugIcon, PlusIcon, Trash2Icon } from "lucide-react";
import { toast } from "sonner";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import { api } from "@/lib/api";
import type { Webshell, WebshellInput } from "@/lib/types";
import { cn } from "@/lib/utils";

function fmtTime(ts: string | null | undefined) {
  if (!ts) return "";
  return new Date(ts).toLocaleString("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });
}

// parseHeaders 把「Key: Value」逐行的文本框内容转成请求头对象。
function parseHeaders(text: string): Record<string, string> {
  const out: Record<string, string> = {};
  for (const raw of text.split("\n")) {
    const line = raw.trim();
    if (!line) continue;
    const idx = line.indexOf(":");
    if (idx <= 0) continue;
    out[line.slice(0, idx).trim()] = line.slice(idx + 1).trim();
  }
  return out;
}

// ShellFormDialog 是新增 / 编辑 webshell 的共用表单。
function ShellFormDialog({
  open,
  onOpenChange,
  initial,
  onSaved,
}: {
  open: boolean;
  onOpenChange: (v: boolean) => void;
  initial: Webshell | null;
  onSaved: (w: Webshell) => void;
}) {
  const [name, setName] = React.useState("");
  const [url, setUrl] = React.useState("");
  const [method, setMethod] = React.useState<"POST" | "GET">("POST");
  const [paramName, setParamName] = React.useState("cmd");
  const [encoding, setEncoding] = React.useState("");
  const [headersText, setHeadersText] = React.useState("");
  const [note, setNote] = React.useState("");
  const [saving, setSaving] = React.useState(false);

  React.useEffect(() => {
    if (!open) return;
    setName(initial?.name ?? "");
    setUrl(initial?.url ?? "");
    setMethod(initial?.method === "GET" ? "GET" : "POST");
    setParamName(initial?.param_name ?? "cmd");
    setEncoding(initial?.encoding ?? "");
    setHeadersText(
      initial
        ? Object.entries(initial.headers ?? {})
            .map(([k, v]) => `${k}: ${v}`)
            .join("\n")
        : "",
    );
    setNote(initial?.note ?? "");
  }, [open, initial]);

  const save = async () => {
    if (!name.trim() || !url.trim()) {
      toast.error("名称和 URL 必填");
      return;
    }
    const input: WebshellInput = {
      name: name.trim(),
      url: url.trim(),
      method,
      param_name: paramName.trim() || "cmd",
      encoding,
      headers: parseHeaders(headersText),
      note: note.trim(),
    };
    setSaving(true);
    try {
      const w = initial ? await api.updateWebshell(initial.id, input) : await api.createWebshell(input);
      toast.success(initial ? "已更新" : "已登记");
      onSaved(w);
      onOpenChange(false);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : String(e));
    } finally {
      setSaving(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{initial ? "编辑 Webshell" : "登记 Webshell"}</DialogTitle>
          <DialogDescription>
            目标上拿到 shell（上传的 webshell 木马 / 命令执行点）后在此登记，即可在平台内直接连接执行命令。
          </DialogDescription>
        </DialogHeader>
        <div className="grid gap-4 py-2">
          <div className="grid gap-2">
            <Label htmlFor="ws-name">名称</Label>
            <Input
              id="ws-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="如：后台 admin.php 菜刀"
            />
          </div>
          <div className="grid gap-2">
            <Label htmlFor="ws-url">执行地址</Label>
            <Input
              id="ws-url"
              value={url}
              onChange={(e) => setUrl(e.target.value)}
              placeholder="http://target/shell.php"
            />
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div className="grid gap-2">
              <Label>请求方式</Label>
              <Select value={method} onValueChange={(v) => setMethod(v as "POST" | "GET")}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="POST">POST（表单参数）</SelectItem>
                  <SelectItem value="GET">GET（URL 参数）</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="grid gap-2">
              <Label htmlFor="ws-param">命令参数名</Label>
              <Input id="ws-param" value={paramName} onChange={(e) => setParamName(e.target.value)} placeholder="cmd" />
            </div>
          </div>
          <div className="grid gap-2">
            <Label htmlFor="ws-headers">附加请求头（每行一条 Key: Value）</Label>
            <Textarea
              id="ws-headers"
              value={headersText}
              onChange={(e) => setHeadersText(e.target.value)}
              rows={3}
              placeholder={"Cookie: PHPSESSID=...\nAuthorization: Basic ..."}
              className="font-mono text-xs"
            />
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div className="grid gap-2">
              <Label>输出编码</Label>
              <Select value={encoding} onValueChange={setEncoding}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="">自动（UTF-8）</SelectItem>
                  <SelectItem value="gbk">GBK</SelectItem>
                  <SelectItem value="gb18030">GB18030</SelectItem>
                  <SelectItem value="big5">Big5</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="grid gap-2">
              <Label htmlFor="ws-note">备注</Label>
              <Input
                id="ws-note"
                value={note}
                onChange={(e) => setNote(e.target.value)}
                placeholder="密码、路径、目标信息…"
              />
            </div>
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={saving}>
            取消
          </Button>
          <Button onClick={save} disabled={saving}>
            {saving && <Loader2Icon className="mr-1 animate-spin" />}
            {initial ? "保存" : "登记"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

type Line = { id: number; kind: "cmd" | "out"; text: string; status?: string; durationMs?: number };

// Terminal 把一条 webshell 的命令历史渲染成终端回显，并支持继续执行。
function Terminal({ shell, onChanged }: { shell: Webshell; onChanged: () => void }) {
  const [lines, setLines] = React.useState<Line[]>([]);
  const [input, setInput] = React.useState("");
  const [running, setRunning] = React.useState(false);
  const [loading, setLoading] = React.useState(false);
  const scrollRef = React.useRef<HTMLDivElement>(null);
  const idRef = React.useRef(0);
  const mk = React.useCallback(
    (kind: Line["kind"], text: string, status?: string, durationMs?: number): Line => ({
      id: ++idRef.current,
      kind,
      text,
      status,
      durationMs,
    }),
    [],
  );

  // 连接：拉取该 shell 的历史命令作为回显。
  React.useEffect(() => {
    let alive = true;
    setLoading(true);
    setLines([]);
    api
      .webshellCommands(shell.id, 200)
      .then((r) => {
        if (!alive) return;
        const hist = [...r.commands].reverse();
        setLines(
          hist.flatMap((c) => [
            mk("cmd", c.command, c.status, c.duration_ms),
            mk("out", c.output || "[无输出]", c.status),
          ]),
        );
      })
      .catch(() => {
        if (!alive) return;
        toast.error("加载执行历史失败");
      })
      .finally(() => alive && setLoading(false));
    return () => {
      alive = false;
    };
  }, [shell.id, mk]);

  // 自动滚到底部（每次渲染后执行，跟随 lines 变化）。
  React.useEffect(() => {
    const el = scrollRef.current;
    if (el) el.scrollTop = el.scrollHeight;
  });

  const run = async (command: string) => {
    setRunning(true);
    setLines((ls) => [...ls, mk("cmd", command)]);
    setInput("");
    try {
      const r = await api.execWebshell(shell.id, command);
      const res = "result" in r ? r.result : r;
      setLines((ls) => [...ls, mk("out", res.output || "[无输出]", res.status, res.duration_ms)]);
      onChanged(); // 刷新列表里的 last_exec_at / last_output
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      setLines((ls) => [...ls, mk("out", msg, "error")]);
      toast.error(msg);
    } finally {
      setRunning(false);
    }
  };

  return (
    <Card className="flex h-full min-h-0 flex-col overflow-hidden border">
      <div className="flex items-center gap-2 border-b px-3 py-2">
        <CircleIcon className={cn("h-3 w-3", loading ? "text-amber-400" : "text-emerald-500")} fill="currentColor" />
        <span className="truncate font-medium text-sm">{shell.name}</span>
        <Badge variant="secondary">{shell.method}</Badge>
        {shell.encoding && <Badge variant="outline">{shell.encoding}</Badge>}
        <span className="truncate text-muted-foreground text-xs">{shell.url}</span>
        <div className="ml-auto flex items-center gap-1">
          <Button variant="ghost" size="sm" onClick={() => setLines([])} title="清屏">
            <EraserIcon className="h-4 w-4" />
          </Button>
        </div>
      </div>
      <div ref={scrollRef} className="min-h-0 flex-1 overflow-auto bg-zinc-950 p-3 font-mono text-xs leading-relaxed">
        {(() => {
          if (loading) {
            return (
              <div className="flex items-center gap-2 text-zinc-500">
                <Loader2Icon className="h-3 w-3 animate-spin" /> 连接中…
              </div>
            );
          }
          if (lines.length === 0) {
            return <div className="text-zinc-600">已连接。输入命令回车执行，如 id、whoami。</div>;
          }
          return lines.map((l) =>
            l.kind === "cmd" ? (
              <div key={l.id} className="mt-2 flex gap-2">
                <span className="shrink-0 text-emerald-500">$</span>
                <span className="whitespace-pre-wrap break-all text-zinc-100">{l.text}</span>
                {l.durationMs !== undefined && <span className="shrink-0 text-zinc-600">{l.durationMs}ms</span>}
              </div>
            ) : (
              <div
                key={l.id}
                className={cn("whitespace-pre-wrap break-all", l.status === "error" ? "text-red-400" : "text-zinc-400")}
              >
                {l.text}
              </div>
            ),
          );
        })()}
        {running && (
          <div className="mt-2 flex items-center gap-2 text-zinc-500">
            <Loader2Icon className="h-3 w-3 animate-spin" /> 执行中…
          </div>
        )}
      </div>
      <form
        className="flex items-center gap-2 border-t bg-zinc-950 px-3 py-2"
        onSubmit={(e) => {
          e.preventDefault();
          const cmd = input.trim();
          if (!cmd || running) return;
          void run(cmd);
        }}
      >
        <span className="shrink-0 font-mono text-emerald-500 text-sm">$</span>
        <input
          value={input}
          onChange={(e) => setInput(e.target.value)}
          disabled={running}
          spellCheck={false}
          placeholder="输入命令，回车执行…"
          className="w-full bg-transparent font-mono text-sm text-zinc-100 outline-none placeholder:text-zinc-600 disabled:opacity-50"
        />
      </form>
    </Card>
  );
}

export default function WebshellPage() {
  const [items, setItems] = React.useState<Webshell[]>([]);
  const [loading, setLoading] = React.useState(true);
  const [selected, setSelected] = React.useState<Webshell | null>(null);
  const [formOpen, setFormOpen] = React.useState(false);
  const [editing, setEditing] = React.useState<Webshell | null>(null);

  const load = React.useCallback(() => {
    setLoading(true);
    api
      .webshells()
      .then((r) => {
        setItems(r.webshells);
        // 选中项同步最新数据（last_output/last_exec_at）。
        setSelected((cur) => {
          if (!cur) return cur;
          const fresh = r.webshells.find((w) => w.id === cur.id);
          return fresh ?? cur;
        });
      })
      .catch(() => toast.error("加载 webshell 列表失败"))
      .finally(() => setLoading(false));
  }, []);

  React.useEffect(() => {
    load();
  }, [load]);

  const remove = (w: Webshell) => {
    if (!window.confirm(`删除「${w.name}」？其命令执行历史将一并清除。`)) return;
    api
      .deleteWebshell(w.id)
      .then(() => {
        toast.success("已删除");
        setSelected((cur) => (cur?.id === w.id ? null : cur));
        load();
      })
      .catch((e) => toast.error(e instanceof Error ? e.message : String(e)));
  };

  return (
    <div className="flex h-full flex-col gap-4">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="font-semibold text-lg">Webshell</h1>
          <p className="text-muted-foreground text-sm">
            Agent 拿到目标 shell 后用 register_webshell 登记，或手动添加；点击左侧条目即可连接执行命令。
          </p>
        </div>
        <Button
          onClick={() => {
            setEditing(null);
            setFormOpen(true);
          }}
        >
          <PlusIcon className="mr-1 h-4 w-4" /> 登记
        </Button>
      </div>

      <div className="grid min-h-0 flex-1 grid-cols-1 gap-4 lg:grid-cols-[340px_1fr]">
        <Card className="flex min-h-0 flex-col overflow-hidden border">
          <div className="border-b px-3 py-2 font-medium text-sm">已获取的 Shell</div>
          <div className="min-h-0 flex-1 overflow-auto">
            {(() => {
              if (loading) {
                return (
                  <div className="flex items-center justify-center gap-2 py-8 text-muted-foreground text-sm">
                    <Loader2Icon className="h-4 w-4 animate-spin" /> 加载中…
                  </div>
                );
              }
              if (items.length === 0) {
                return (
                  <div className="px-4 py-8 text-center text-muted-foreground text-sm">
                    暂无 Webshell。
                    <br />
                    点击右上角「登记」添加，或等 Agent 在任务中拿到 shell 后自动登记。
                  </div>
                );
              }
              return items.map((w) => (
                <div
                  key={w.id}
                  className={cn("border-b last:border-b-0", selected?.id === w.id ? "bg-accent" : "hover:bg-muted/60")}
                >
                  <button type="button" onClick={() => setSelected(w)} className="w-full px-3 pt-2.5 text-left">
                    <div className="flex items-center gap-2">
                      <span className="truncate font-medium text-sm">{w.name}</span>
                      <Badge variant="secondary">{w.method}</Badge>
                      {w.encoding && <Badge variant="outline">{w.encoding}</Badge>}
                      {w.task_name && (
                        <Badge variant="outline" className="ml-auto shrink-0">
                          任务 {w.task_name}
                        </Badge>
                      )}
                    </div>
                    <div className="mt-0.5 truncate font-mono text-muted-foreground text-xs">{w.url}</div>
                  </button>
                  <div className="flex items-center gap-2 px-3 pb-2.5 text-muted-foreground text-xs">
                    <span className="shrink-0">参数 {w.param_name}</span>
                    {w.last_exec_at && <span className="truncate">最近执行 {fmtTime(w.last_exec_at)}</span>}
                    <span className="ml-auto flex shrink-0 items-center gap-1">
                      <Button
                        variant="ghost"
                        size="sm"
                        className="h-6 px-1.5"
                        onClick={() => {
                          setEditing(w);
                          setFormOpen(true);
                        }}
                        title="编辑"
                      >
                        <PencilIcon className="h-3.5 w-3.5" />
                      </Button>
                      <Button
                        variant="ghost"
                        size="sm"
                        className="h-6 px-1.5 text-destructive hover:text-destructive"
                        onClick={() => remove(w)}
                        title="删除"
                      >
                        <Trash2Icon className="h-3.5 w-3.5" />
                      </Button>
                    </span>
                  </div>
                </div>
              ));
            })()}
          </div>
        </Card>

        <div className="min-h-[480px]">
          {selected ? (
            <Terminal shell={selected} onChanged={load} />
          ) : (
            <Card className="flex h-full min-h-0 flex-col items-center justify-center gap-3 border border-dashed text-muted-foreground">
              <PlugIcon className="h-8 w-8" />
              <p className="text-sm">选择一个 Webshell 连接，或在左侧登记新的</p>
            </Card>
          )}
        </div>
      </div>

      <ShellFormDialog
        open={formOpen}
        onOpenChange={setFormOpen}
        initial={editing}
        onSaved={(w) => {
          setSelected(w); // 新建/编辑后自动连接
          load();
        }}
      />
    </div>
  );
}
