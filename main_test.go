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

func TestGreetCommand(t *testing.T) {
	req := CommandRequest{Command: "greet", Name: "TestUser"}
	resp := processCommand(req)

	if !resp.Success {
		t.Errorf("Expected success=true, got false. Result: %v", resp.Result)
	}
	result, ok := resp.Result.(string)
	if !ok {
		t.Fatalf("Expected string result, got %T", resp.Result)
	}
	if result == "" {
		t.Error("Expected non-empty result")
	}
	t.Logf("Greet result: %s", result)
}

func TestCalcCommands(t *testing.T) {
	tests := []struct {
		name     string
		a, b     float64
		op       string
		wantPass bool
	}{
		{"addition", 10, 20, "add", true},
		{"subtraction", 50, 30, "sub", true},
		{"multiplication", 7, 8, "mul", true},
		{"division", 100, 4, "div", true},
		{"division by zero", 10, 0, "div", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := CommandRequest{Command: "calc", A: tt.a, B: tt.b, Op: tt.op}
			resp := processCommand(req)
			if resp.Success != tt.wantPass {
				t.Errorf("Expected success=%v, got %v. Result: %v", tt.wantPass, resp.Success, resp.Result)
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
	t.Logf("Time result: %v", resp.Result)
}

func TestSysInfoCommand(t *testing.T) {
	resp := processCommand(CommandRequest{Command: "sysinfo"})
	if !resp.Success {
		t.Errorf("Expected success, got: %v", resp.Result)
	}
	t.Logf("SysInfo result: %v", resp.Result)
}

func TestTransformCommand(t *testing.T) {
	tests := []struct {
		text, mode, expected string
	}{
		{"hello", "upper", "HELLO"},
		{"WORLD", "lower", "world"},
		{"abc", "reverse", "cba"},
	}

	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			req := CommandRequest{Command: "transform", Text: tt.text, Mode: tt.mode}
			resp := processCommand(req)
			if !resp.Success {
				t.Errorf("Expected success, got: %v", resp.Result)
			}
			result, _ := resp.Result.(string)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
			t.Logf("Transform %s(%s) = %s", tt.mode, tt.text, result)
		})
	}
}

func TestUnknownCommand(t *testing.T) {
	resp := processCommand(CommandRequest{Command: "foobar"})
	if resp.Success {
		t.Error("Expected failure for unknown command")
	}
	t.Logf("Unknown command result: %v", resp.Result)
}

func TestHTTPExecuteEndpoint(t *testing.T) {
	payload := CommandRequest{Command: "greet", Name: "HTTPTest"}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/execute", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handleExecute(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp CommandResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if !resp.Success {
		t.Errorf("Expected success, got: %v", resp.Result)
	}
	t.Logf("HTTP result: %v", resp.Result)
}

func TestHTTPHealthEndpoint(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()

	handleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var result map[string]interface{}
	json.NewDecoder(w.Body).Decode(&result)
	if result["status"] != "ok" {
		t.Errorf("Expected status=ok, got %v", result["status"])
	}
	t.Logf("Health: %v", result)
}
