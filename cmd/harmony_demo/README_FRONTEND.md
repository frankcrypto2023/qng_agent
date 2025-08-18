# Harmony GPT-OSS 前端

这是一个专门为 Harmony GPT-OSS 协议设计的现代化 Web 前端界面。

## 特性

- 🎨 **现代化 UI**: 使用 React + TypeScript + Tailwind CSS 构建
- 💬 **多模式支持**: 基础模式、工具模式、流式模式
- 🔄 **实时流式响应**: 支持 Server-Sent Events 流式聊天
- 📱 **响应式设计**: 适配桌面和移动设备
- 🎯 **对话管理**: 支持多对话、删除对话、模式切换
- ⚡ **高性能**: 使用 Vite 构建，快速加载

## 技术栈

- **前端框架**: React 18 + TypeScript
- **构建工具**: Vite
- **样式框架**: Tailwind CSS
- **图标库**: Lucide React
- **状态管理**: React Hooks

## 目录结构

```
frontend/
├── src/
│   ├── components/          # React 组件
│   │   ├── ChatInput.tsx    # 聊天输入组件
│   │   ├── ChatMessage.tsx  # 聊天消息组件
│   │   └── Sidebar.tsx      # 侧边栏组件
│   ├── api.ts              # API 客户端
│   ├── types.ts            # TypeScript 类型定义
│   ├── App.tsx             # 主应用组件
│   ├── main.tsx            # 应用入口
│   └── index.css           # 全局样式
├── package.json            # 依赖配置
├── vite.config.ts          # Vite 配置
├── tailwind.config.js      # Tailwind 配置
└── tsconfig.json           # TypeScript 配置
```

## 开发

### 安装依赖

```bash
cd cmd/harmony_demo/frontend
npm install
```

### 开发模式

```bash
npm run dev
```

### 构建生产版本

```bash
npm run build
```

## 使用

### 启动服务器

```bash
# 编译 Go 程序
go build -o harmony_demo cmd/harmony_demo/main.go cmd/harmony_demo/web_server.go

# 启动 Web 服务器
./harmony_demo -mode web -port 8081
```

### 访问界面

打开浏览器访问: http://localhost:8081

## 功能说明

### 对话模式

1. **基础模式**: 简单的对话模式，适合一般问答
2. **工具模式**: 支持函数调用，适合复杂任务
3. **流式模式**: 实时流式响应，体验更流畅

### 对话管理

- 创建新对话
- 切换对话
- 删除对话
- 自动保存对话历史

### 界面特性

- 深色主题
- 响应式布局
- 实时消息显示
- 加载状态指示
- 错误处理

## API 集成

前端通过以下 API 与后端通信：

- `POST /chat`: 普通聊天
- `POST /stream`: 流式聊天

## 自定义

### 修改样式

编辑 `src/index.css` 和 `tailwind.config.js` 来自定义样式。

### 添加组件

在 `src/components/` 目录下添加新的 React 组件。

### 修改 API

编辑 `src/api.ts` 来修改 API 调用逻辑。

## 故障排除

### 构建失败

1. 检查 Node.js 版本 (推荐 16+)
2. 删除 `node_modules` 并重新安装依赖
3. 检查 TypeScript 错误

### 静态资源 404

1. 确保前端已构建 (`npm run build`)
2. 检查 `dist/` 目录是否存在
3. 重启 Go 服务器

### API 连接失败

1. 确保后端服务器正在运行
2. 检查端口配置
3. 查看浏览器控制台错误信息

## 贡献

欢迎提交 Issue 和 Pull Request 来改进这个前端界面！
