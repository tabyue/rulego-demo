# RuleGo Demo v2 - 规则引擎命令路由 + 自然语言意图识别

基于 [RuleGo](https://github.com/rulego/rulego) v0.35.0 实现的规则引擎示例项目。

**v2 核心特性：支持自然语言输入！** 直接说"走几步"、"帮我算3加5"、"讲个笑话"，规则引擎通过 JS 脚本做关键词匹配和意图识别，路由到对应处理逻辑。

## 功能演示

### 自然语言模式 🗣️

| 你说的话 | 识别意图 | 响应示例 |
|---------|---------|---------|
| `你好` / `hello` / `早上好` | greet | 你好呀！很高兴见到你 👋 |
| `走几步` / `跳个舞` / `表演一个` | action | 🚶 走几步... 踢踏踢踏~ 哒哒哒！ |
| `现在几点` / `今天星期几` | time | 🕐 现在是 2026-04-02 📅 星期四 |
| `帮我算3加5` / `10乘20等于多少` | calc | 🔢 3 + 5 = 8 |
| `系统信息` / `看看状态` | sysinfo | 💻 主机: xxx, 引擎: RuleGo v0.35.0 |
| `转换大写` / `反转文本` | transform | 🔄 大写: HELLO, 反转: oGeluR |
| `执行 echo hello` / `运行 date` | exec | ⚡ 执行: echo hello → hello |
| `讲个笑话` / `逗我一下` | joke | 😄 为什么程序员分不清万圣节和圣诞节... |
| `帮助` / `怎么用` | help | 📖 使用指南 |
| 其他任意输入 | unknown | 🤔 抱歉，没理解... |

### 结构化命令模式 ⚡

| 命令 | 说明 | 示例 |
|------|------|------|
| `greet` | 生成问候语 | `{"command":"greet","name":"Alice"}` |
| `calc` | 四则运算 | `{"command":"calc","a":10,"b":20,"op":"add"}` |
| `time` | 服务器时间 | `{"command":"time"}` |
| `sysinfo` | 系统信息 | `{"command":"sysinfo"}` |
| `transform` | 文本转换 | `{"command":"transform","text":"hello","mode":"upper"}` |

## 架构

```
用户输入 (自然语言 / 结构化命令)
    │
    ├─ 自然语言 ──→ nlp_chain.json
    │                  │
    │              jsSwitch (关键词意图识别)
    │                  │
    │    ┌──────┬──────┼──────┬──────┬──────┬──────┬──────┬──────┐
    │    ▼      ▼      ▼      ▼      ▼      ▼      ▼      ▼      ▼
    │  greet action  time   calc  sysinfo transform exec  joke  help
    │    │      │      │      │      │      │       │      │      │
    │    └──────┴──────┴──────┴──────┴──────┴───────┴──────┴──────┘
    │                          │
    │                     log (记录)
    │
    └─ 结构化命令 ──→ router_chain.json
                       │
                   jsSwitch (精确匹配)
                       │
           ┌───────┬───┴───┬─────────┬───────────┐
           ▼       ▼       ▼         ▼           ▼
         greet   calc    time    sysinfo    transform
```

**所有业务逻辑都在 JSON 规则链的 JS 脚本中实现，修改规则无需改 Go 代码！**

## 快速开始

### 前置条件

- Go 1.21+
- Git

### 1. 克隆并运行

```bash
git clone https://github.com/tabyue/rulego-demo.git
cd rulego-demo
go mod tidy
```

### 2. 四种运行模式

```bash
# HTTP 服务器（默认）- 启动 Web UI + REST API
go run main.go

# 自然语言对话模式 - 终端直接聊天
go run main.go chat

# 结构化命令模式 - 传统 CLI
go run main.go cli

# 内置测试 - 自动跑 20 个测试用例
go run main.go test
```

## API 接口

### POST /api/chat - 自然语言输入（推荐）

```bash
# 走几步
curl -X POST http://localhost:8080/api/chat \
  -H "Content-Type: application/json" \
  -d '{"input":"走几步"}'

# 帮我算
curl -X POST http://localhost:8080/api/chat \
  -H "Content-Type: application/json" \
  -d '{"input":"帮我算3加5"}'

# 讲笑话
curl -X POST http://localhost:8080/api/chat \
  -H "Content-Type: application/json" \
  -d '{"input":"讲个笑话"}'

# 执行命令
curl -X POST http://localhost:8080/api/chat \
  -H "Content-Type: application/json" \
  -d '{"input":"执行 echo hello world"}'
```

### POST /api/execute - 结构化命令

```bash
curl -X POST http://localhost:8080/api/execute \
  -H "Content-Type: application/json" \
  -d '{"command":"calc","a":42,"b":8,"op":"mul"}'
```

### POST /api/exec - 直接执行 Shell 命令

```bash
curl -X POST http://localhost:8080/api/exec \
  -H "Content-Type: application/json" \
  -d '{"cmd":"echo hello"}'
```

> ⚠️ Shell 执行有白名单安全控制，仅允许 echo/date/whoami/hostname 等安全命令。

### GET /api/health - 健康检查

```bash
curl http://localhost:8080/api/health
```

## 服务器部署

### 方式一：直接编译部署

```bash
# 交叉编译 Linux 版
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o rulego-demo .

# 上传
scp rulego-demo rules/ user@server:/opt/rulego-demo/

# 在服务器上
cd /opt/rulego-demo
chmod +x rulego-demo
./rulego-demo test              # 先跑测试
PORT=8080 nohup ./rulego-demo > app.log 2>&1 &
```

### 方式二：Docker

```bash
docker build -t rulego-demo .
docker run -d -p 8080:8080 --name rulego-demo rulego-demo
```

### 方式三：systemd 服务

```ini
# /etc/systemd/system/rulego-demo.service
[Unit]
Description=RuleGo Demo
After=network.target

[Service]
Type=simple
WorkingDirectory=/opt/rulego-demo
ExecStart=/opt/rulego-demo/rulego-demo
Restart=always
Environment=PORT=8080

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl enable --now rulego-demo
```

## 运行测试

```bash
# Go 单元测试（包含 NLP + 结构化 + HTTP + Shell 执行测试）
go test -v ./...

# 内置集成测试（20 个用例）
go run main.go test
```

## 项目结构

```
rulego-demo/
├── main.go              # 主程序（HTTP + Chat + CLI + Test 四种模式）
├── main_test.go         # Go 单元测试
├── go.mod / go.sum
├── Dockerfile
├── .gitignore
├── README.md
└── rules/
    ├── nlp_chain.json       # ⭐ 自然语言意图识别链（关键词匹配 → 9 种意图）
    ├── router_chain.json    # 结构化命令路由链
    ├── greeting_chain.json  # 问候子链
    └── calc_chain.json      # 计算子链
```

## 如何扩展

想增加新的自然语言意图？只需编辑 `rules/nlp_chain.json`：

1. 在 `jsSwitch` 的 `rules` 数组中添加新规则（关键词 + 路由名）
2. 添加新的 `jsTransform` 节点处理该意图
3. 添加对应的 `connection`

**无需修改任何 Go 代码，无需重新编译！**（运行时可通过 API 热更新规则链）

## License

MIT
