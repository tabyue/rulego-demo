# RuleGo Demo - 规则引擎命令路由示例

基于 [RuleGo](https://github.com/rulego/rulego) 实现的规则引擎示例项目。输入不同的字符串命令，通过 **规则链路由** 分发到不同的处理逻辑（JS 脚本计算、文本转换、系统信息获取等）。

## 功能概览

| 命令 | 说明 | 示例输入 |
|------|------|----------|
| `greet` | 生成问候语 | `{"command":"greet","name":"Alice"}` |
| `calc` | 四则运算 | `{"command":"calc","a":10,"b":20,"op":"add"}` |
| `time` | 获取服务器时间 | `{"command":"time"}` |
| `sysinfo` | 获取系统信息 | `{"command":"sysinfo"}` |
| `transform` | 文本转换（大写/小写/反转/长度） | `{"command":"transform","text":"hello","mode":"upper"}` |
| 其他 | 返回未知命令提示 | `{"command":"foobar"}` |

## 架构说明

```
输入字符串
    │
    ▼
┌─────────────┐
│  jsSwitch   │  ← 根据 command 字段路由
│  命令路由器  │
└─────┬───────┘
      │
  ┌───┴───┬───────┬─────────┬───────────┬─────────┐
  ▼       ▼       ▼         ▼           ▼         ▼
greet   calc    time    sysinfo   transform  unknown
  │       │       │         │           │         │
  ▼       ▼       ▼         ▼           ▼         ▼
┌─────────────────────────────────────────────────────┐
│              log 节点 - 统一记录结果                  │
└─────────────────────────────────────────────────────┘
```

## 快速开始

### 前置条件

- Go 1.21+
- Git

### 1. 克隆项目

```bash
git clone https://github.com/tabyue/rulego-demo.git
cd rulego-demo
```

### 2. 安装依赖

```bash
go mod tidy
```

### 3. 运行方式

项目支持三种运行模式：

#### 模式一：HTTP 服务器（推荐用于服务器部署）

```bash
go run main.go
```

启动后访问 `http://localhost:8080` 打开 Web UI，或使用 curl 调用 API。

**自定义端口：**
```bash
PORT=3000 go run main.go
```

#### 模式二：内置测试（快速验证）

```bash
go run main.go test
```

自动运行 10 个预设测试用例，输出通过/失败结果。

#### 模式三：交互式 CLI

```bash
go run main.go cli
```

在终端交互式输入命令进行测试。

## API 接口

### POST /api/execute

执行命令：

```bash
# 问候
curl -X POST http://localhost:8080/api/execute \
  -H "Content-Type: application/json" \
  -d '{"command":"greet","name":"World"}'

# 计算
curl -X POST http://localhost:8080/api/execute \
  -H "Content-Type: application/json" \
  -d '{"command":"calc","a":42,"b":8,"op":"mul"}'

# 获取时间
curl -X POST http://localhost:8080/api/execute \
  -H "Content-Type: application/json" \
  -d '{"command":"time"}'

# 系统信息
curl -X POST http://localhost:8080/api/execute \
  -H "Content-Type: application/json" \
  -d '{"command":"sysinfo"}'

# 文本转换
curl -X POST http://localhost:8080/api/execute \
  -H "Content-Type: application/json" \
  -d '{"command":"transform","text":"Hello RuleGo","mode":"reverse"}'

# 未知命令
curl -X POST http://localhost:8080/api/execute \
  -H "Content-Type: application/json" \
  -d '{"command":"foobar"}'
```

### GET /api/health

健康检查：

```bash
curl http://localhost:8080/api/health
```

## 服务器部署

### 方式一：直接编译部署

```bash
# 编译（Linux 服务器）
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o rulego-demo .

# 上传到服务器
scp rulego-demo rules/ user@server:/opt/rulego-demo/

# 在服务器上运行
ssh user@server
cd /opt/rulego-demo
chmod +x rulego-demo

# 先跑测试
./rulego-demo test

# 启动服务
PORT=8080 nohup ./rulego-demo > app.log 2>&1 &
```

### 方式二：Docker 部署

```bash
# 构建镜像
docker build -t rulego-demo .

# 运行容器
docker run -d -p 8080:8080 --name rulego-demo rulego-demo

# 查看日志
docker logs -f rulego-demo
```

### 方式三：systemd 服务（Linux 推荐）

创建服务文件 `/etc/systemd/system/rulego-demo.service`：

```ini
[Unit]
Description=RuleGo Demo Service
After=network.target

[Service]
Type=simple
User=www-data
WorkingDirectory=/opt/rulego-demo
ExecStart=/opt/rulego-demo/rulego-demo
Restart=always
RestartSec=5
Environment=PORT=8080

[Install]
WantedBy=multi-user.target
```

启用并启动服务：

```bash
sudo systemctl daemon-reload
sudo systemctl enable rulego-demo
sudo systemctl start rulego-demo
sudo systemctl status rulego-demo
```

## 运行测试

### 单元测试

```bash
go test -v ./...
```

### 内置集成测试

```bash
go run main.go test
```

### curl 一键验证脚本

```bash
#!/bin/bash
BASE="http://localhost:8080"

echo "=== Health Check ==="
curl -s $BASE/api/health | jq .

echo -e "\n=== Greet ==="
curl -s -X POST $BASE/api/execute -H "Content-Type: application/json" \
  -d '{"command":"greet","name":"Test"}' | jq .

echo -e "\n=== Calc ==="
curl -s -X POST $BASE/api/execute -H "Content-Type: application/json" \
  -d '{"command":"calc","a":100,"b":3,"op":"div"}' | jq .

echo -e "\n=== Time ==="
curl -s -X POST $BASE/api/execute -H "Content-Type: application/json" \
  -d '{"command":"time"}' | jq .

echo -e "\n=== Transform ==="
curl -s -X POST $BASE/api/execute -H "Content-Type: application/json" \
  -d '{"command":"transform","text":"RuleGo Demo","mode":"reverse"}' | jq .

echo -e "\n=== Unknown ==="
curl -s -X POST $BASE/api/execute -H "Content-Type: application/json" \
  -d '{"command":"unknown_cmd"}' | jq .

echo -e "\nAll tests done!"
```

## 项目结构

```
rulego-demo/
├── main.go              # 主程序（HTTP服务器 + CLI + 内置测试）
├── main_test.go         # Go 单元测试
├── go.mod               # Go 模块定义
├── go.sum               # 依赖校验
├── Dockerfile           # Docker 构建文件
├── .gitignore
├── README.md
└── rules/               # 规则链定义（JSON）
    ├── router_chain.json    # 主路由链 - jsSwitch 根据命令分发
    ├── greeting_chain.json  # 问候处理链
    └── calc_chain.json      # 计算处理链
```

## 规则链说明

### router_chain.json（核心）

主路由链使用 `jsSwitch` 节点根据输入的 `command` 字段值将消息路由到不同的 `jsTransform` 处理节点：

- **s1 (jsSwitch)**: 解析 command，返回对应路由名
- **s2~s7 (jsTransform)**: 各命令的具体处理逻辑（JS 脚本实现）
- **s8 (log)**: 统一记录处理结果

所有业务逻辑通过 JSON 配置的 JS 脚本实现，**无需修改 Go 代码即可调整业务规则**。

## License

MIT
