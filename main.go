package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/rulego/rulego"
	"github.com/rulego/rulego/api/types"
)

const (
	routerChainID = "router_chain"
	greetChainID  = "greeting_chain"
	calcChainID   = "calc_chain"
	nlpChainID    = "nlp_chain"
	defaultPort   = "8080"
)

// 允许执行的安全命令白名单
var safeCommands = map[string]bool{
	"echo": true, "date": true, "whoami": true, "hostname": true,
	"uname": true, "ls": true, "dir": true, "pwd": true, "cat": true,
	"ping": true, "ipconfig": true, "ifconfig": true, "ip": true,
	"go": true, "git": true, "uptime": true, "free": true, "df": true,
	"env": true, "set": true, "type": true, "which": true, "where": true,
}

// CommandRequest 是 HTTP API 的请求体
type CommandRequest struct {
	Command string  `json:"command"`           // 命令名称: greet, calc, time, sysinfo, transform
	Input   string  `json:"input,omitempty"`   // 自然语言输入
	Name    string  `json:"name,omitempty"`    // greet 命令用
	A       float64 `json:"a,omitempty"`       // calc 命令用
	B       float64 `json:"b,omitempty"`       // calc 命令用
	Op      string  `json:"op,omitempty"`      // calc 命令: add, sub, mul, div
	Text    string  `json:"text,omitempty"`    // transform 命令用
	Mode    string  `json:"mode,omitempty"`    // transform 命令: upper, lower, reverse, length
	Cmd     string  `json:"cmd,omitempty"`     // exec 命令用
}

// CommandResponse 是 HTTP API 的响应体
type CommandResponse struct {
	Success bool        `json:"success"`
	Command string      `json:"command"`
	Intent  string      `json:"intent,omitempty"`
	Result  interface{} `json:"result"`
	Time    string      `json:"time"`
}

func main() {
	fmt.Println("========================================")
	fmt.Println("  RuleGo Demo - 规则引擎命令路由示例")
	fmt.Println("  支持自然语言输入 🗣️")
	fmt.Println("========================================")

	// 初始化规则引擎
	initRuleEngines()

	// 判断运行模式
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "cli":
			runCLI()
		case "chat":
			runChat()
		case "test":
			runTests()
		default:
			fmt.Printf("Unknown mode: %s\n", os.Args[1])
			fmt.Println("Usage: rulego-demo [chat|cli|test]")
			fmt.Println("  (no args) - HTTP server mode")
			fmt.Println("  chat      - Natural language chat mode")
			fmt.Println("  cli       - Structured command mode")
			fmt.Println("  test      - Run built-in tests")
		}
	} else {
		runHTTPServer()
	}
}

// initRuleEngines 加载所有规则链并创建规则引擎实例
func initRuleEngines() {
	chains := map[string]string{
		routerChainID: "rules/router_chain.json",
		greetChainID:  "rules/greeting_chain.json",
		calcChainID:   "rules/calc_chain.json",
		nlpChainID:    "rules/nlp_chain.json",
	}

	config := rulego.NewConfig()
	config.OnDebug = func(chainId, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
		if err != nil {
			log.Printf("[DEBUG] chain=%s flow=%s node=%s relation=%s err=%v", chainId, flowType, nodeId, relationType, err)
		}
	}

	for id, file := range chains {
		data, err := os.ReadFile(file)
		if err != nil {
			log.Fatalf("Failed to load rule chain %s from %s: %v", id, file, err)
		}
		_, err = rulego.New(id, data, rulego.WithConfig(config))
		if err != nil {
			log.Fatalf("Failed to create rule engine %s: %v", id, err)
		}
		fmt.Printf("  ✓ Loaded rule chain: %s\n", id)
	}
	fmt.Println()
}

// processNLP 通过自然语言规则链处理输入
func processNLP(input string) CommandResponse {
	payload, _ := json.Marshal(map[string]string{"input": input})
	metaData := types.NewMetadata()
	metaData.PutValue("hostname", getHostname())
	metaData.PutValue("timestamp", time.Now().Format(time.RFC3339))

	msg := types.NewMsg(0, "NLP_INPUT", types.JSON, metaData, string(payload))

	engine, ok := rulego.Get(nlpChainID)
	if !ok {
		return CommandResponse{Success: false, Result: "NLP rule engine not found", Time: time.Now().Format(time.RFC3339)}
	}

	var resultMsg types.RuleMsg
	var wg sync.WaitGroup
	wg.Add(1)
	engine.OnMsg(msg, types.WithEndFunc(func(ctx types.RuleContext, endMsg types.RuleMsg, err error) {
		resultMsg = endMsg
		wg.Done()
	}))
	wg.Wait()

	var resultData map[string]interface{}
	json.Unmarshal([]byte(resultMsg.GetData()), &resultData)

	result := "No result"
	if r, ok := resultData["result"]; ok {
		result = fmt.Sprintf("%v", r)
	}
	intent := ""
	if i, ok := resultData["matched_intent"]; ok {
		intent = fmt.Sprintf("%v", i)
	}

	// 如果匹配到 exec 意图且有命令，实际执行它
	if intent == "exec" {
		if shellCmd, ok := resultData["shell_cmd"]; ok && shellCmd != "" {
			cmdStr := fmt.Sprintf("%v", shellCmd)
			execResult := executeShellCommand(cmdStr)
			result = fmt.Sprintf("⚡ 执行: %s\n\n%s", cmdStr, execResult)
		}
	}

	return CommandResponse{
		Success: true,
		Command: "nlp",
		Intent:  intent,
		Result:  result,
		Time:    time.Now().Format(time.RFC3339),
	}
}

// processCommand 通过结构化规则链处理命令
func processCommand(req CommandRequest) CommandResponse {
	// 如果有自然语言输入，走 NLP 链
	if req.Input != "" {
		return processNLP(req.Input)
	}

	payload, _ := json.Marshal(req)
	metaData := types.NewMetadata()
	metaData.PutValue("hostname", getHostname())
	metaData.PutValue("timestamp", time.Now().Format(time.RFC3339))

	msg := types.NewMsg(0, "COMMAND", types.JSON, metaData, string(payload))

	engine, ok := rulego.Get(routerChainID)
	if !ok {
		return CommandResponse{Success: false, Command: req.Command, Result: "Rule engine not found", Time: time.Now().Format(time.RFC3339)}
	}

	var resultMsg types.RuleMsg
	var wg sync.WaitGroup
	wg.Add(1)
	engine.OnMsg(msg, types.WithEndFunc(func(ctx types.RuleContext, endMsg types.RuleMsg, err error) {
		resultMsg = endMsg
		wg.Done()
	}))
	wg.Wait()

	var resultData map[string]interface{}
	json.Unmarshal([]byte(resultMsg.GetData()), &resultData)

	result := "No result"
	if r, ok := resultData["result"]; ok {
		result = fmt.Sprintf("%v", r)
	}
	status := true
	if s, ok := resultData["status"]; ok && s == "error" {
		status = false
	}

	return CommandResponse{Success: status, Command: req.Command, Result: result, Time: time.Now().Format(time.RFC3339)}
}

// executeShellCommand 安全执行 shell 命令
func executeShellCommand(cmdStr string) string {
	parts := strings.Fields(cmdStr)
	if len(parts) == 0 {
		return "❌ 空命令"
	}

	// 安全检查
	cmdName := strings.ToLower(parts[0])
	if !safeCommands[cmdName] {
		return fmt.Sprintf("🚫 命令 \"%s\" 不在安全白名单中\n✅ 允许的命令: echo, date, whoami, hostname, uname, ls, dir, pwd, ping, go, git, uptime, df ...", parts[0])
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", append([]string{"/C"}, parts...)...)
	} else {
		cmd = exec.Command(parts[0], parts[1:]...)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("❌ 执行失败: %v\n%s", err, string(output))
	}
	result := strings.TrimSpace(string(output))
	if result == "" {
		result = "(命令已执行，无输出)"
	}
	return "✅ " + result
}

// ==================== HTTP Server 模式 ====================

func runHTTPServer() {
	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/api/execute", handleExecute)
	http.HandleFunc("/api/chat", handleChat)
	http.HandleFunc("/api/exec", handleExec)
	http.HandleFunc("/api/health", handleHealth)

	fmt.Printf("🚀 HTTP Server started on http://0.0.0.0:%s\n", port)
	fmt.Println("API endpoints:")
	fmt.Printf("  POST /api/chat     - Natural language input (自然语言)\n")
	fmt.Printf("  POST /api/execute  - Structured command\n")
	fmt.Printf("  POST /api/exec     - Execute shell command\n")
	fmt.Printf("  GET  /api/health   - Health check\n")
	fmt.Printf("  GET  /              - Web UI\n")
	fmt.Println()

	log.Fatal(http.ListenAndServe("0.0.0.0:"+port, nil))
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, indexHTML)
}

func handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Input string `json:"input"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Input == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(CommandResponse{Success: false, Result: "请提供 input 字段", Time: time.Now().Format(time.RFC3339)})
		return
	}
	resp := processNLP(body.Input)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func handleExec(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Cmd string `json:"cmd"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Cmd == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(CommandResponse{Success: false, Result: "请提供 cmd 字段", Time: time.Now().Format(time.RFC3339)})
		return
	}
	result := executeShellCommand(body.Cmd)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(CommandResponse{
		Success: true, Command: "exec", Intent: "exec",
		Result: result, Time: time.Now().Format(time.RFC3339),
	})
}

func handleExecute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req CommandRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(CommandResponse{Success: false, Result: "Invalid JSON: " + err.Error(), Time: time.Now().Format(time.RFC3339)})
		return
	}
	resp := processCommand(req)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok", "version": "2.0.0", "engine": "RuleGo",
		"features": []string{"structured_command", "natural_language", "shell_exec"},
		"time": time.Now().Format(time.RFC3339), "go": runtime.Version(),
	})
}

// ==================== Chat 模式（自然语言交互） ====================

func runChat() {
	fmt.Println("🗣️  自然语言对话模式 - 直接输入中文/英文，我来理解你的意思！")
	fmt.Println("💡 试试: \"你好\" / \"走几步\" / \"几点了\" / \"帮我算3加5\" / \"讲个笑话\"")
	fmt.Println("📝 输入 \"帮助\" 查看所有功能，\"quit\" 退出")
	fmt.Println(strings.Repeat("─", 50))

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("\n🗣️  你: ")
		if !scanner.Scan() {
			break
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}
		if input == "quit" || input == "exit" || input == "退出" {
			fmt.Println("👋 再见！")
			return
		}

		resp := processNLP(input)
		fmt.Printf("🤖 [%s]: %v\n", resp.Intent, resp.Result)
	}
}

// ==================== CLI 模式（结构化命令） ====================

func runCLI() {
	fmt.Println("CLI Mode - 结构化命令模式")
	fmt.Println("可用命令: greet, calc, time, sysinfo, transform, quit")
	fmt.Println("💡 提示: 使用 'chat' 模式可以用自然语言交互")
	fmt.Println("---")

	for {
		fmt.Print("\n> 输入命令: ")
		var cmd string
		fmt.Scanln(&cmd)

		if cmd == "quit" || cmd == "exit" {
			fmt.Println("Bye!")
			return
		}

		var req CommandRequest
		switch cmd {
		case "greet":
			fmt.Print("  输入名字: ")
			fmt.Scanln(&req.Name)
			req.Command = "greet"
		case "calc":
			fmt.Print("  输入数字a: ")
			fmt.Scanln(&req.A)
			fmt.Print("  输入操作(add/sub/mul/div): ")
			fmt.Scanln(&req.Op)
			fmt.Print("  输入数字b: ")
			fmt.Scanln(&req.B)
			req.Command = "calc"
		case "time":
			req.Command = "time"
		case "sysinfo":
			req.Command = "sysinfo"
		case "transform":
			fmt.Print("  输入文本: ")
			fmt.Scanln(&req.Text)
			fmt.Print("  输入模式(upper/lower/reverse/length): ")
			fmt.Scanln(&req.Mode)
			req.Command = "transform"
		default:
			req.Command = cmd
		}

		resp := processCommand(req)
		fmt.Printf("  ✅ Result: %v\n", resp.Result)
	}
}

// ==================== Test 模式 ====================

func runTests() {
	fmt.Println("Running built-in tests...")
	fmt.Println()

	type testCase struct {
		name    string
		mode    string // "cmd" or "nlp"
		req     CommandRequest
		nlpText string
	}

	tests := []testCase{
		// 结构化命令测试
		{name: "Cmd: Greet", mode: "cmd", req: CommandRequest{Command: "greet", Name: "RuleGo"}},
		{name: "Cmd: Calc add", mode: "cmd", req: CommandRequest{Command: "calc", A: 10, B: 20, Op: "add"}},
		{name: "Cmd: Calc multiply", mode: "cmd", req: CommandRequest{Command: "calc", A: 7, B: 8, Op: "mul"}},
		{name: "Cmd: Time", mode: "cmd", req: CommandRequest{Command: "time"}},
		{name: "Cmd: Transform upper", mode: "cmd", req: CommandRequest{Command: "transform", Text: "hello", Mode: "upper"}},
		{name: "Cmd: Unknown", mode: "cmd", req: CommandRequest{Command: "foobar"}},

		// 自然语言测试
		{name: "NLP: 你好", mode: "nlp", nlpText: "你好"},
		{name: "NLP: 走几步", mode: "nlp", nlpText: "走几步"},
		{name: "NLP: 走两步看看", mode: "nlp", nlpText: "走两步看看"},
		{name: "NLP: 跳个舞", mode: "nlp", nlpText: "跳个舞"},
		{name: "NLP: 现在几点", mode: "nlp", nlpText: "现在几点了"},
		{name: "NLP: 3加5", mode: "nlp", nlpText: "帮我算一下3加5"},
		{name: "NLP: 10乘20", mode: "nlp", nlpText: "10乘20等于多少"},
		{name: "NLP: 系统信息", mode: "nlp", nlpText: "看看系统信息"},
		{name: "NLP: 讲个笑话", mode: "nlp", nlpText: "讲个笑话吧"},
		{name: "NLP: 帮助", mode: "nlp", nlpText: "帮助"},
		{name: "NLP: 执行echo", mode: "nlp", nlpText: "执行 echo hello world"},
		{name: "NLP: hello", mode: "nlp", nlpText: "hello there"},
		{name: "NLP: 大写转换", mode: "nlp", nlpText: "转换大写"},
		{name: "NLP: unknown", mode: "nlp", nlpText: "量子纠缠是什么"},
	}

	passed := 0
	failed := 0
	for i, tt := range tests {
		var resp CommandResponse
		if tt.mode == "cmd" {
			resp = processCommand(tt.req)
		} else {
			resp = processNLP(tt.nlpText)
		}

		status := "✅"
		if !resp.Success {
			if tt.mode == "cmd" && tt.req.Command == "foobar" {
				// expected failure
			} else {
				status = "❌"
				failed++
				fmt.Printf("%s FAIL | #%d %s\n   Result: %v\n\n", status, i+1, tt.name, resp.Result)
				continue
			}
		}
		passed++

		detail := ""
		if tt.mode == "nlp" {
			resultStr := fmt.Sprintf("%v", resp.Result)
			if len(resultStr) > 60 {
				resultStr = resultStr[:60] + "..."
			}
			detail = fmt.Sprintf(" [intent=%s] %s", resp.Intent, resultStr)
		} else {
			detail = fmt.Sprintf(" %v", resp.Result)
		}
		fmt.Printf("%s #%-2d %s%s\n", status, i+1, tt.name, detail)
	}

	fmt.Println()
	fmt.Println(strings.Repeat("=", 50))
	fmt.Printf("Total: %d | Passed: %d | Failed: %d\n", passed+failed, passed, failed)
	fmt.Println(strings.Repeat("=", 50))
}

// ==================== Utils ====================

func getHostname() string {
	h, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return h
}

// ==================== HTML UI ====================

const indexHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>RuleGo Demo v2</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { font-family: -apple-system, 'Segoe UI', sans-serif; background: #0f172a; color: #e2e8f0; min-height: 100vh; }
        .container { max-width: 800px; margin: 0 auto; padding: 40px 20px; }
        h1 { text-align: center; font-size: 2em; margin-bottom: 4px; background: linear-gradient(135deg, #60a5fa, #a78bfa); -webkit-background-clip: text; -webkit-text-fill-color: transparent; }
        .subtitle { text-align: center; color: #94a3b8; margin-bottom: 30px; }
        .tabs { display: flex; gap: 4px; margin-bottom: 20px; }
        .tab { flex: 1; padding: 10px; text-align: center; border-radius: 8px 8px 0 0; cursor: pointer; background: #1e293b; color: #94a3b8; border: 1px solid #334155; border-bottom: none; transition: all 0.2s; }
        .tab.active { background: #334155; color: #e2e8f0; font-weight: 600; }
        .panel { display: none; }
        .panel.active { display: block; }
        .card { background: #1e293b; border-radius: 0 0 12px 12px; padding: 24px; margin-bottom: 20px; border: 1px solid #334155; border-top: none; }
        .card2 { background: #1e293b; border-radius: 12px; padding: 24px; margin-bottom: 20px; border: 1px solid #334155; }
        .card2 h2 { color: #60a5fa; margin-bottom: 16px; font-size: 1.1em; }
        label { display: block; color: #94a3b8; margin-bottom: 4px; font-size: 0.9em; }
        select, input, textarea, button { width: 100%; padding: 10px 14px; border-radius: 8px; border: 1px solid #334155; background: #0f172a; color: #e2e8f0; font-size: 1em; margin-bottom: 12px; }
        textarea { resize: vertical; min-height: 50px; font-family: inherit; }
        select:focus, input:focus, textarea:focus { outline: none; border-color: #60a5fa; }
        .btn { background: linear-gradient(135deg, #3b82f6, #8b5cf6); border: none; cursor: pointer; font-weight: 600; transition: opacity 0.2s; }
        .btn:hover { opacity: 0.9; }
        .btn-sm { width: auto; padding: 6px 14px; font-size: 0.85em; background: #334155; border-radius: 20px; cursor: pointer; border: 1px solid #475569; }
        .btn-sm:hover { background: #475569; }
        .examples { display: flex; gap: 8px; flex-wrap: wrap; margin-bottom: 16px; }
        .params { display: none; }
        .params.active { display: block; }
        .result-box { background: #0f172a; border: 1px solid #334155; border-radius: 8px; padding: 16px; min-height: 80px; white-space: pre-wrap; font-family: 'Fira Code', monospace; font-size: 0.9em; line-height: 1.6; }
        .result-box.success { border-color: #22c55e; }
        .result-box.error { border-color: #ef4444; }
        .grid { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; }
        .chat-input-row { display: flex; gap: 8px; }
        .chat-input-row input { flex: 1; margin-bottom: 0; }
        .chat-input-row button { width: auto; padding: 10px 20px; margin-bottom: 0; }
        .intent-badge { display: inline-block; padding: 2px 8px; border-radius: 12px; font-size: 0.75em; background: #3b82f6; color: white; margin-bottom: 8px; }
    </style>
</head>
<body>
    <div class="container">
        <h1>RuleGo Demo v2</h1>
        <p class="subtitle">自然语言 + 结构化命令，双模式规则引擎</p>

        <div class="tabs">
            <div class="tab active" onclick="switchTab('chat')">🗣️ 自然语言</div>
            <div class="tab" onclick="switchTab('cmd')">⚡ 结构化命令</div>
        </div>

        <!-- 自然语言面板 -->
        <div id="panel-chat" class="panel active">
            <div class="card">
                <div class="examples">
                    <button class="btn-sm" onclick="chatSend('你好')">你好</button>
                    <button class="btn-sm" onclick="chatSend('走几步')">走几步</button>
                    <button class="btn-sm" onclick="chatSend('跳个舞')">跳个舞</button>
                    <button class="btn-sm" onclick="chatSend('现在几点')">几点了</button>
                    <button class="btn-sm" onclick="chatSend('帮我算3加5')">3加5</button>
                    <button class="btn-sm" onclick="chatSend('讲个笑话')">笑话</button>
                    <button class="btn-sm" onclick="chatSend('系统信息')">系统信息</button>
                    <button class="btn-sm" onclick="chatSend('执行 echo hello')">执行命令</button>
                    <button class="btn-sm" onclick="chatSend('帮助')">帮助</button>
                </div>
                <div class="chat-input-row">
                    <input type="text" id="chatInput" placeholder='试试输入 "走几步" / "帮我算10乘20" / "讲个笑话"...' onkeypress="if(event.key==='Enter')chatGo()">
                    <button class="btn" onclick="chatGo()">发送</button>
                </div>
            </div>
        </div>

        <!-- 结构化命令面板 -->
        <div id="panel-cmd" class="panel">
            <div class="card">
                <div class="examples">
                    <button class="btn-sm" onclick="quickCmd('greet')">Greet</button>
                    <button class="btn-sm" onclick="quickCmd('calc')">Calc</button>
                    <button class="btn-sm" onclick="quickCmd('time')">Time</button>
                    <button class="btn-sm" onclick="quickCmd('sysinfo')">SysInfo</button>
                    <button class="btn-sm" onclick="quickCmd('transform')">Transform</button>
                </div>
                <label>Command</label>
                <select id="command" onchange="toggleParams()">
                    <option value="greet">greet - 问候</option>
                    <option value="calc">calc - 计算</option>
                    <option value="time">time - 获取时间</option>
                    <option value="sysinfo">sysinfo - 系统信息</option>
                    <option value="transform">transform - 文本转换</option>
                </select>
                <div id="params-greet" class="params active"><label>Name</label><input type="text" id="name" value="RuleGo"></div>
                <div id="params-calc" class="params"><div class="grid"><div><label>A</label><input type="number" id="a" value="10"></div><div><label>B</label><input type="number" id="b" value="20"></div></div><label>Op</label><select id="op"><option value="add">add</option><option value="sub">sub</option><option value="mul">mul</option><option value="div">div</option></select></div>
                <div id="params-transform" class="params"><label>Text</label><input type="text" id="text" value="Hello RuleGo"><label>Mode</label><select id="mode"><option value="upper">upper</option><option value="lower">lower</option><option value="reverse">reverse</option><option value="length">length</option></select></div>
                <button class="btn" onclick="cmdGo()">🚀 Execute</button>
            </div>
        </div>

        <div class="card2">
            <h2>📋 Result</h2>
            <div id="intentBadge" class="intent-badge" style="display:none"></div>
            <div id="result" class="result-box">输入内容后点击发送...</div>
        </div>
    </div>

    <script>
        function switchTab(tab) {
            document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
            document.querySelectorAll('.panel').forEach(p => p.classList.remove('active'));
            event.target.classList.add('active');
            document.getElementById('panel-' + tab).classList.add('active');
        }
        function toggleParams() {
            document.querySelectorAll('.params').forEach(p => p.classList.remove('active'));
            const el = document.getElementById('params-' + document.getElementById('command').value);
            if (el) el.classList.add('active');
        }
        function chatSend(text) { document.getElementById('chatInput').value = text; chatGo(); }
        function chatGo() {
            const input = document.getElementById('chatInput').value.trim();
            if (!input) return;
            showResult('thinking...');
            fetch('/api/chat', { method:'POST', headers:{'Content-Type':'application/json'}, body: JSON.stringify({input}) })
                .then(r => r.json()).then(d => { showResult(d.result, d.success, d.intent); })
                .catch(e => showResult('Error: ' + e.message, false));
        }
        function quickCmd(cmd) {
            const presets = { greet:{command:'greet',name:'RuleGo'}, calc:{command:'calc',a:42,b:8,op:'mul'}, time:{command:'time'}, sysinfo:{command:'sysinfo'}, transform:{command:'transform',text:'Hello RuleGo',mode:'reverse'} };
            sendCmd(presets[cmd]);
        }
        function cmdGo() {
            const cmd = document.getElementById('command').value;
            let body = { command: cmd };
            if (cmd==='greet') body.name = document.getElementById('name').value;
            else if (cmd==='calc') { body.a=parseFloat(document.getElementById('a').value); body.b=parseFloat(document.getElementById('b').value); body.op=document.getElementById('op').value; }
            else if (cmd==='transform') { body.text=document.getElementById('text').value; body.mode=document.getElementById('mode').value; }
            sendCmd(body);
        }
        function sendCmd(body) {
            showResult('Processing...');
            fetch('/api/execute', { method:'POST', headers:{'Content-Type':'application/json'}, body: JSON.stringify(body) })
                .then(r => r.json()).then(d => showResult(d.result, d.success, d.intent))
                .catch(e => showResult('Error: ' + e.message, false));
        }
        function showResult(text, success, intent) {
            const box = document.getElementById('result');
            const badge = document.getElementById('intentBadge');
            box.className = 'result-box' + (success===true?' success':success===false?' error':'');
            box.textContent = typeof text === 'object' ? JSON.stringify(text, null, 2) : text;
            if (intent) { badge.style.display = 'inline-block'; badge.textContent = 'intent: ' + intent; }
            else { badge.style.display = 'none'; }
        }
    </script>
</body>
</html>`
