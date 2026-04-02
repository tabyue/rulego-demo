package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMain(m *testing.M) {
	initRuleEngines()
	m.Run()
}

// ==================== 结构化命令测试 ====================

func TestGreetCommand(t *testing.T) {
	resp := processCommand(CommandRequest{Command: "greet", Name: "TestUser"})
	if !resp.Success {
		t.Errorf("Expected success, got: %v", resp.Result)
	}
	t.Logf("Greet result: %v", resp.Result)
}

func TestCalcCommands(t *testing.T) {
	tests := []struct {
		name string
		a, b float64
		op   string
	}{
		{"addition", 10, 20, "add"},
		{"subtraction", 50, 30, "sub"},
		{"multiplication", 7, 8, "mul"},
		{"division", 100, 4, "div"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := processCommand(CommandRequest{Command: "calc", A: tt.a, B: tt.b, Op: tt.op})
			if !resp.Success {
				t.Errorf("Expected success, got: %v", resp.Result)
			}
			t.Logf("%s: %v", tt.name, resp.Result)
		})
	}
}

func TestTimeCommand(t *testing.T) {
	resp := processCommand(CommandRequest{Command: "time"})
	if !resp.Success {
		t.Errorf("Expected success, got: %v", resp.Result)
	}
	t.Logf("Time: %v", resp.Result)
}

func TestSysInfoCommand(t *testing.T) {
	resp := processCommand(CommandRequest{Command: "sysinfo"})
	if !resp.Success {
		t.Errorf("Expected success, got: %v", resp.Result)
	}
	t.Logf("SysInfo: %v", resp.Result)
}

func TestTransformCommand(t *testing.T) {
	tests := []struct{ text, mode, expected string }{
		{"hello", "upper", "HELLO"},
		{"WORLD", "lower", "world"},
		{"abc", "reverse", "cba"},
	}
	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			resp := processCommand(CommandRequest{Command: "transform", Text: tt.text, Mode: tt.mode})
			if !resp.Success {
				t.Errorf("Expected success, got: %v", resp.Result)
			}
			t.Logf("Transform %s(%s) = %v", tt.mode, tt.text, resp.Result)
		})
	}
}

// ==================== 自然语言测试 ====================

func TestNLPGreet(t *testing.T) {
	tests := []string{"你好", "hello", "Hi there", "早上好"}
	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			resp := processNLP(input)
			if !resp.Success || resp.Intent != "greet" {
				t.Errorf("Expected intent=greet, got intent=%s result=%v", resp.Intent, resp.Result)
			}
			t.Logf("[%s] %v", resp.Intent, resp.Result)
		})
	}
}

func TestNLPAction(t *testing.T) {
	tests := []string{"走几步", "走两步看看", "跳个舞", "表演一个"}
	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			resp := processNLP(input)
			if !resp.Success || resp.Intent != "action" {
				t.Errorf("Expected intent=action, got intent=%s result=%v", resp.Intent, resp.Result)
			}
			t.Logf("[%s] %v", resp.Intent, resp.Result)
		})
	}
}

func TestNLPTime(t *testing.T) {
	resp := processNLP("现在几点了")
	if resp.Intent != "time" {
		t.Errorf("Expected intent=time, got %s", resp.Intent)
	}
	t.Logf("[%s] %v", resp.Intent, resp.Result)
}

func TestNLPCalc(t *testing.T) {
	tests := []string{"帮我算3加5", "10乘20等于多少", "100除以4"}
	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			resp := processNLP(input)
			if resp.Intent != "calc" {
				t.Errorf("Expected intent=calc, got %s", resp.Intent)
			}
			t.Logf("[%s] %v", resp.Intent, resp.Result)
		})
	}
}

func TestNLPJoke(t *testing.T) {
	resp := processNLP("讲个笑话吧")
	if resp.Intent != "joke" {
		t.Errorf("Expected intent=joke, got %s", resp.Intent)
	}
	t.Logf("[%s] %v", resp.Intent, resp.Result)
}

func TestNLPHelp(t *testing.T) {
	resp := processNLP("帮助")
	if resp.Intent != "help" {
		t.Errorf("Expected intent=help, got %s", resp.Intent)
	}
	t.Logf("[%s] %v", resp.Intent, resp.Result)
}

func TestNLPExec(t *testing.T) {
	resp := processNLP("执行 echo hello")
	if resp.Intent != "exec" {
		t.Errorf("Expected intent=exec, got %s", resp.Intent)
	}
	t.Logf("[%s] %v", resp.Intent, resp.Result)
}

func TestNLPUnknown(t *testing.T) {
	resp := processNLP("量子纠缠是什么")
	if resp.Intent != "unknown" {
		t.Errorf("Expected intent=unknown, got %s", resp.Intent)
	}
	t.Logf("[%s] %v", resp.Intent, resp.Result)
}

// ==================== Shell执行测试 ====================

func TestShellExecSafe(t *testing.T) {
	result := executeShellCommand("echo hello")
	if result == "" {
		t.Error("Expected non-empty result")
	}
	t.Logf("Shell exec: %s", result)
}

func TestShellExecBlocked(t *testing.T) {
	result := executeShellCommand("rm -rf /")
	if !contains(result, "不在安全白名单") {
		t.Errorf("Expected blocked message, got: %s", result)
	}
	t.Logf("Blocked: %s", result)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsCheck(s, substr))
}

func containsCheck(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// ==================== HTTP 接口测试 ====================

func TestHTTPChatEndpoint(t *testing.T) {
	body, _ := json.Marshal(map[string]string{"input": "走几步"})
	req := httptest.NewRequest(http.MethodPost, "/api/chat", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handleChat(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}
	var resp CommandResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Intent != "action" {
		t.Errorf("Expected intent=action, got %s", resp.Intent)
	}
	t.Logf("HTTP chat: [%s] %v", resp.Intent, resp.Result)
}

func TestHTTPExecuteEndpoint(t *testing.T) {
	payload := CommandRequest{Command: "greet", Name: "HTTPTest"}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/execute", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handleExecute(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}
	var resp CommandResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if !resp.Success {
		t.Errorf("Expected success, got: %v", resp.Result)
	}
	t.Logf("HTTP execute: %v", resp.Result)
}

func TestHTTPHealthEndpoint(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()
	handleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}
	var result map[string]interface{}
	json.NewDecoder(w.Body).Decode(&result)
	if result["status"] != "ok" {
		t.Errorf("Expected status=ok, got %v", result["status"])
	}
	t.Logf("Health: %v", result)
}
