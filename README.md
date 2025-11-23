# Combot Server Go

> 🚀 基于小智 AI go-server 的魔改版本，增加了 MCP 功能支持

本项目是 [小智 AI go-server](https://github.com/AnimeAIChat/xiaozhi-server-go) 的增强版本，在原有基础上进行了大量优化和功能扩展。

<p align="center">
  <img src="https://github.com/user-attachments/assets/aa1e2f26-92d3-4d16-a74a-68232f34cca3" alt="Xiaozhi Architecture" width="600">
</p>

---

## 🎯 主要特性

### 🏗️ 核心架构优化

* **完整的用户系统**
  * 用户注册与设备绑定
  * 完善的错误处理机制
  * 统一的日志系统（基于 Logrus）
  
* **稳定的 WebSocket 连接**
  * EOF 读取异常修复
  * 退出聆听问题优化
  * Context 管理优化

* **对话管理系统**
  * 对话历史记录
  * 角色管理与切换
  * 多种对话模式支持

### 🎤 语音处理能力

* **豆包 ASR 深度集成**
  * 重写豆包 ASR 模块
  * 流式语音识别
  * ASR 结果优化处理
  
* **多格式音频支持**
  * PCM 格式
  * Opus 格式
  * 音频播放优化

### 🔧 OTA 固件管理

* 固件版本管理
* OTA 下发流程
* 自动升级支持

### 🤖 MCP 协议支持

* 服务端 MCP Server
* 本地 MCP 工具调用
* 客户端 MCP 透传
* 资源池管理机制

### 📦 核心目录结构

```
src/
├── core/                  # 核心功能模块
│   ├── mcp/              # MCP 协议实现
│   ├── auth/             # 认证模块
│   ├── chat/             # 对话管理
│   └── pool/             # 资源池管理
├── dao/                  # 数据访问层
├── handlers/             # HTTP 处理器
├── service/              # 业务逻辑层
├── models/               # 数据模型
└── middleware/           # 中间件
```


---

## ✨ 完整功能列表

### 基础功能

* [x] WebSocket 长连接支持
* [x] PCM / Opus 格式语音对话
* [x] 豆包流式 ASR 语音识别
* [x] EdgeTTS / 豆包 TTS 语音合成
* [x] OpenAI API / Ollama 大模型对接
* [x] 智谱 API 智能识图
* [x] auto / manual / realtime 三种对话模式
* [x] 实时打断支持
* [x] 对话历史记录
* [x] 角色管理与切换
* [x] 音乐播放控制

### 系统功能

* [x] 用户注册与认证
* [x] 设备绑定管理
* [x] OTA 固件升级
* [x] SQLite 本地数据库
* [x] 统一错误处理
* [x] Logrus 日志系统
* [x] Swagger API 文档

### MCP 协议

* [x] 服务端 MCP Server
* [x] 本地 MCP 工具调用
* [x] 客户端 MCP 透传
* [x] 资源池管理
* [x] 天气查询工具
* [x] 地图服务集成

### 客户端支持

* [x] ESP32 小智客户端
* [x] Python 客户端
* [x] Android 客户端

---

## 🚀 快速开始

### 1. 环境要求

* Go 1.24.2+
* SQLite 3
* （Windows 用户）CGO + Opus 库

### 2. 克隆项目

```bash
git clone https://github.com/harryin777/combot-server-go.git
cd combot-server-go
```

### 3. 配置文件

```bash
# 复制配置模板
cp config.yaml .config.yaml

# 编辑配置文件，填入您的 API 密钥和服务地址
# - ASR/TTS/LLM 模型配置
# - WebSocket 地址
# - MCP 服务配置（如需要）
```

### 4. 安装依赖

```bash
go mod tidy
```

### 5. 运行服务

```bash
go run ./src/main.go
```

服务将在以下端口启动：

* HTTP/WebSocket: `8080`
* Swagger 文档: `http://localhost:8080/swagger/index.html`

---

## 💬 MCP 配置指南

MCP 功能详细配置请参考：**[src/core/mcp/README.md](src/core/mcp/README.md)**

### 快速配置示例

在 `.config.yaml` 中配置 MCP 服务：

```yaml
mcp:
  enabled: true
  servers:
    - name: "weather"
      type: "local"
      command: "npx"
      args: ["-y", "@modelcontextprotocol/server-weather"]
  
  client_mcp:
    enabled: true
    timeout: 30s
```

---

## 🛠️ 开发指南

### Windows 开发环境配置

安装 [MSYS2](https://www.msys2.org/)，然后执行：

```bash
pacman -Syu
pacman -S mingw-w64-x86_64-gcc mingw-w64-x86_64-opus mingw-w64-x86_64-pkg-config
```

设置环境变量：

```bash
set PKG_CONFIG_PATH=C:\msys64\mingw64\lib\pkgconfig
set CGO_ENABLED=1
```

### 更新 Swagger 文档

修改 API 后需要更新文档：

```bash
cd src
swag init -g main.go
```

### 编译发布版本

```bash
go build -o combot-server ./src/main.go
```

---

## 📚 技术栈

* **语言**: Go 1.24.2+
* **数据库**: SQLite
* **日志**: Logrus
* **音频**: Opus 编解码
* **WebSocket**: Gorilla WebSocket
* **大模型**: OpenAI API / Ollama
* **语音**: 豆包 ASR/TTS、EdgeTTS
* **视觉**: 智谱 API

---

## 📖 项目文档

* [Swagger API 文档](http://localhost:8080/swagger/index.html)
* [MCP 配置指南](src/core/mcp/README.md)
* [CentOS 部署指南](Centos_Guide.md)

---

## 🤝 贡献者

感谢所有为本项目做出贡献的开发者！

主要贡献者：
主要贡献者：

* **玄凤科技** - MCP 功能实现、资源池管理
* **harry** - 基础架构、语音处理、稳定性优化
* **kalicyh** - CI/CD、构建优化
* **zhonghuihong** - 功能增强与优化

---

## 📄 开源协议

本项目遵循 **Apache 2.0** 协议开源。

---

## 🙏 致谢

* 感谢 [小智 AI](https://github.com/AnimeAIChat/xiaozhi-server-go) 提供的优秀基础框架
* 感谢 [虾哥的 ESP32 项目](https://github.com/78/xiaozhi-esp32) 开创的生态
* 感谢所有贡献者和使用者的支持

