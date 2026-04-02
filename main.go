package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/rulego/rulego"
	"github.com/rulego/rulego/api/types"
)

const (
	routerChainID  = "router_chain"
	greetChainID   = "greeting_chain"
	calcChainID    = "calc_chain"
	defaultPort    = "8080"
)

// CommandRequest 是 HTTP API 的请求体
type CommandRequest struct {
	Command string      `json:"command"`           // 命令名称: greet, calc, time, sysinfo, transform
	Name    string      `json:"name,omitempty"`    // greet 命令用
	A       float64     `json:"a,omitempty"`       // calc 命令用
	B       float64     `json:"b,omitempty"`       // calc 命令用
	Op      string      `json:"op,omitempty"`      // calc 命令: add, sub, mul, div
	Text    string      `json:"text,omitempty"`    // transform 命令用
	Mode    string      `json:"mode,omitempty"`    // transform 命令: upper, lower, reverse, length
}

// CommandResponse 是 HTTP API 的响应体
type CommandResponse struct {
	Success bool        `json:"success"`
	Command string      `json:"command"`
	Result  interface{} `json:"result"`
	Time    string      `json:"time"`
}

func main() {
	fmt.Println("========================================")
	fmt.Println("   RuleGo Demo - 规则引擎命令路由示例")
	fmt.Println("========================================")

	// 初始化规则引擎
	initRuleEngines()

	// 判断运行模式
	if len(os.Args) > 1 && os.Args[1] == "cli" {
		runCLI()
	} else if len(os.Args) > 1 && os.Args[1] == "test" {
		runTests()
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
	}

	config := rulego.NewConfig()
	// 设置调试回调
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

// processCommand 通过规则引擎处理命令
func processCommand(req CommandRequest) CommandResponse {
	// 构造消息
	payload, _ := json.Marshal(req)
	metaData := types.NewMetadata()
	metaData.PutValue("hostname", getHostname())
	metaData.PutValue("timestamp", time.Now().Format(time.RFC3339))

	msg := types.NewMsg(0, "COMMAND", types.JSON, metaData, string(payload))

	engine, ok := rulego.Get(routerChainID)
	if !ok {
		return CommandResponse{
			Success: false,
			Command: req.Command,
			Result:  "Rule engine not found",
			Time:    time.Now().Format(time.RFC3339),
		}
	}

	var resultMsg types.RuleMsg
	var wg sync.WaitGroup
	wg.Add(1)

	engine.OnMsg(msg, types.WithEndFunc(func(ctx types.RuleContext, endMsg types.RuleMsg, err error) {
		resultMsg = endMsg
		wg.Done()
	}))
	wg.Wait()

	// 解析结果
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

	return CommandResponse{
		Success: status,
		Command: req.Command,
		Result:  result,
		Time:    time.Now().Format(time.RFC3339),
	}
}

// ==================== HTTP Server 模式 ====================

func runHTTPServer() {
	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/api/execute", handleExecute)
	http.HandleFunc("/api/health", handleHealth)

	fmt.Printf("🚀 HTTP Server started on http://0.0.0.0:%s\n", port)
	fmt.Println("API endpoints:")
	fmt.Printf("  POST http://localhost:%s/api/execute  - Execute command\n", port)
	fmt.Printf("  GET  http://localhost:%s/api/health   - Health check\n", port)
	fmt.Printf("  GET  http://localhost:%s/             - Web UI\n", port)
	fmt.Println()

	log.Fatal(http.ListenAndServe("0.0.0.0:"+port, nil))
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, indexHTML)
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
		json.NewEncoder(w).Encode(CommandResponse{
			Success: false,
			Result:  "Invalid JSON: " + err.Error(),
			Time:    time.Now().Format(time.RFC3339),
		})
		return
	}

	resp := processCommand(req)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "ok",
		"version": "1.0.0",
		"engine":  "RuleGo",
		"time":    time.Now().Format(time.RFC3339),
		"go":      runtime.Version(),
	})
}

// ==================== CLI 模式 ====================

func runCLI() {
	fmt.Println("CLI Mode - 输入命令进行测试")
	fmt.Println("可用命令: greet, calc, time, sysinfo, transform, quit")
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

	tests := []struct {
		name string
		req  CommandRequest
	}{
		{
			name: "Test 1: Greet command",
			req:  CommandRequest{Command: "greet", Name: "RuleGo"},
		},
		{
			name: "Test 2: Calc add",
			req:  CommandRequest{Command: "calc", A: 10, B: 20, Op: "add"},
		},
		{
			name: "Test 3: Calc multiply",
			req:  CommandRequest{Command: "calc", A: 7, B: 8, Op: "mul"},
		},
		{
			name: "Test 4: Calc division by zero",
			req:  CommandRequest{Command: "calc", A: 10, B: 0, Op: "div"},
		},
		{
			name: "Test 5: Get time",
			req:  CommandRequest{Command: "time"},
		},
		{
			name: "Test 6: System info",
			req:  CommandRequest{Command: "sysinfo"},
		},
		{
			name: "Test 7: Transform to uppercase",
			req:  CommandRequest{Command: "transform", Text: "hello world", Mode: "upper"},
		},
		{
			name: "Test 8: Transform reverse",
			req:  CommandRequest{Command: "transform", Text: "RuleGo", Mode: "reverse"},
		},
		{
			name: "Test 9: Transform length",
			req:  CommandRequest{Command: "transform", Text: "Hello RuleGo", Mode: "length"},
		},
		{
			name: "Test 10: Unknown command",
			req:  CommandRequest{Command: "foobar"},
		},
	}

	passed := 0
	failed := 0
	for _, tt := range tests {
		resp := processCommand(tt.req)
		status := "✅ PASS"
		if !resp.Success && tt.req.Command != "foobar" {
			status = "❌ FAIL"
			failed++
		} else {
			passed++
		}
		fmt.Printf("%s | %s\n", status, tt.name)
		fmt.Printf("   Command: %s -> Result: %v\n\n", tt.req.Command, resp.Result)
	}

	fmt.Println("========================================")
	fmt.Printf("Total: %d | Passed: %d | Failed: %d\n", passed+failed, passed, failed)
	fmt.Println("========================================")
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
    <title>RuleGo Demo</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { font-family: -apple-system, 'Segoe UI', sans-serif; background: #0f172a; color: #e2e8f0; min-height: 100vh; }
        .container { max-width: 800px; margin: 0 auto; padding: 40px 20px; }
        h1 { text-align: center; font-size: 2em; margin-bottom: 8px; background: linear-gradient(135deg, #60a5fa, #a78bfa); -webkit-background-clip: text; -webkit-text-fill-color: transparent; }
        .subtitle { text-align: center; color: #94a3b8; margin-bottom: 40px; }
        .card { background: #1e293b; border-radius: 12px; padding: 24px; margin-bottom: 20px; border: 1px solid #334155; }
        .card h2 { color: #60a5fa; margin-bottom: 16px; font-size: 1.1em; }
        label { display: block; color: #94a3b8; margin-bottom: 4px; font-size: 0.9em; }
        select, input, button { width: 100%; padding: 10px 14px; border-radius: 8px; border: 1px solid #334155; background: #0f172a; color: #e2e8f0; font-size: 1em; margin-bottom: 12px; }
        select:focus, input:focus { outline: none; border-color: #60a5fa; }
        button { background: linear-gradient(135deg, #3b82f6, #8b5cf6); border: none; cursor: pointer; font-weight: 600; transition: opacity 0.2s; }
        button:hover { opacity: 0.9; }
        .params { display: none; }
        .params.active { display: block; }
        .result-box { background: #0f172a; border: 1px solid #334155; border-radius: 8px; padding: 16px; min-height: 80px; white-space: pre-wrap; font-family: 'Fira Code', monospace; font-size: 0.9em; }
        .result-box.success { border-color: #22c55e; }
        .result-box.error { border-color: #ef4444; }
        .grid { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; }
        .examples { display: flex; gap: 8px; flex-wrap: wrap; margin-bottom: 16px; }
        .examples button { width: auto; padding: 6px 14px; font-size: 0.85em; background: #334155; border-radius: 20px; }
        .examples button:hover { background: #475569; }
    </style>
</head>
<body>
    <div class="container">
        <h1>RuleGo Demo</h1>
        <p class="subtitle">输入不同命令，通过规则引擎路由到不同处理逻辑</p>
        
        <div class="card">
            <h2>📡 Command Center</h2>
            <div class="examples">
                <button onclick="quickTest('greet')">Greet</button>
                <button onclick="quickTest('calc')">Calc</button>
                <button onclick="quickTest('time')">Time</button>
                <button onclick="quickTest('sysinfo')">SysInfo</button>
                <button onclick="quickTest('transform')">Transform</button>
                <button onclick="quickTest('unknown')">Unknown</button>
            </div>
            
            <label>Command</label>
            <select id="command" onchange="toggleParams()">
                <option value="greet">greet - 问候</option>
                <option value="calc">calc - 计算</option>
                <option value="time">time - 获取时间</option>
                <option value="sysinfo">sysinfo - 系统信息</option>
                <option value="transform">transform - 文本转换</option>
            </select>
            
            <div id="params-greet" class="params active">
                <label>Name</label>
                <input type="text" id="name" placeholder="Your name" value="RuleGo">
            </div>
            <div id="params-calc" class="params">
                <div class="grid">
                    <div><label>A</label><input type="number" id="a" value="10"></div>
                    <div><label>B</label><input type="number" id="b" value="20"></div>
                </div>
                <label>Operation</label>
                <select id="op">
                    <option value="add">add (+)</option>
                    <option value="sub">sub (-)</option>
                    <option value="mul">mul (x)</option>
                    <option value="div">div (/)</option>
                </select>
            </div>
            <div id="params-transform" class="params">
                <label>Text</label>
                <input type="text" id="text" placeholder="Enter text" value="Hello RuleGo">
                <label>Mode</label>
                <select id="mode">
                    <option value="upper">upper - 转大写</option>
                    <option value="lower">lower - 转小写</option>
                    <option value="reverse">reverse - 反转</option>
                    <option value="length">length - 长度</option>
                </select>
            </div>
            
            <button onclick="execute()">🚀 Execute</button>
        </div>
        
        <div class="card">
            <h2>📋 Result</h2>
            <div id="result" class="result-box">Click "Execute" to see result...</div>
        </div>
    </div>
    
    <script>
        function toggleParams() {
            document.querySelectorAll('.params').forEach(p => p.classList.remove('active'));
            const cmd = document.getElementById('command').value;
            const el = document.getElementById('params-' + cmd);
            if (el) el.classList.add('active');
        }
        
        function quickTest(cmd) {
            const presets = {
                greet: {command:'greet',name:'RuleGo'},
                calc: {command:'calc',a:42,b:8,op:'mul'},
                time: {command:'time'},
                sysinfo: {command:'sysinfo'},
                transform: {command:'transform',text:'Hello RuleGo',mode:'reverse'},
                unknown: {command:'foobar'}
            };
            sendRequest(presets[cmd]);
        }
        
        function execute() {
            const cmd = document.getElementById('command').value;
            let body = { command: cmd };
            switch(cmd) {
                case 'greet': body.name = document.getElementById('name').value; break;
                case 'calc':
                    body.a = parseFloat(document.getElementById('a').value);
                    body.b = parseFloat(document.getElementById('b').value);
                    body.op = document.getElementById('op').value;
                    break;
                case 'transform':
                    body.text = document.getElementById('text').value;
                    body.mode = document.getElementById('mode').value;
                    break;
            }
            sendRequest(body);
        }
        
        async function sendRequest(body) {
            const box = document.getElementById('result');
            box.className = 'result-box';
            box.textContent = 'Processing...';
            try {
                const res = await fetch('/api/execute', {
                    method: 'POST',
                    headers: {'Content-Type':'application/json'},
                    body: JSON.stringify(body)
                });
                const data = await res.json();
                box.className = 'result-box ' + (data.success ? 'success' : 'error');
                box.textContent = JSON.stringify(data, null, 2);
            } catch(e) {
                box.className = 'result-box error';
                box.textContent = 'Error: ' + e.message;
            }
        }
    </script>
</body>
</html>`
