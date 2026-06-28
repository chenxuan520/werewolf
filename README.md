# AI Werewolf Workbench

[![CI](https://github.com/chenxuan520/werewolf/actions/workflows/ci.yml/badge.svg)](https://github.com/chenxuan520/werewolf/actions/workflows/ci.yml)

> 一个本地优先的 AI 狼人杀工作台。你可以开纯 AI 观战局，也可以自己占 1 个座位，和其余 AI 一起跑完整场对局，并查看历史与回放。

当前仓库已经有一版可跑的最小闭环：

- **观战**：纯 AI 对局，支持半自动 / 全自动 / 手动逐步三档推进。
- **人机**：1 真人 + 其余 AI，系统会自动推进到下一个真人决策点。
- **查看过程**：Lobby / Live Room / History / Replay 四个工作台页签。
- **真实模型接入**：支持 OpenAI 兼容 endpoint / token / model 配置。

---

## 当前规则范围

首版当前实现的是固定小板子，不是规则编辑器：

- `classic-6`：2 狼 + 2 民 + 预言家 + 女巫
- `classic-7`：2 狼 + 3 民 + 预言家 + 女巫
- `classic-8`：3 狼 + 3 民 + 预言家 + 女巫
- `classic-9`：3 狼 + 3 民 + 预言家 + 女巫 + 猎人

白天流程是：

1. 主发言轮
2. 回应轮
3. 投票

夜晚当前覆盖：狼人、预言家、女巫、猎人（仅 9 人板）、基础结算。

---

## 快速开始

### 1) 准备本地 AI 配置

```bash
cp config/ai-presets.demo.yaml config/ai-presets.yaml
```

然后编辑 `config/ai-presets.yaml`，填入你自己的：

- `endpoint`
- `token`
- `model`

> `config/ai-presets.yaml` 已被 `.gitignore` 忽略，不会进仓库。

### 2) 启后端

```bash
cd backend && go run ./cmd/server
```

默认监听：`http://127.0.0.1:18131`

### 3) 启前端

```bash
cd frontend && npm install && npm run dev
```

默认打开：`http://127.0.0.1:5174`

前端会把 `/api` 代理到 `config/app.json` 里的 `frontend.apiTarget`。

---

## 观战 / 人机模式说明

### 纯 AI 观战

- **半自动（默认）**：系统会自动跑完整个夜晚和白天，直到**当天投票结算后**暂停。
- **全自动**：整场持续自动推进到结束。
- **手动逐步**：每次 AI 行动都要手动点一次“下一步”。

### 1 真人 + AI

- 你轮到自己时输入发言或技能动作。
- 系统会把其余 AI 自动推进完，再停在下一个真人输入点。

---

## 开发验证

```bash
cd backend && go test ./...
cd frontend && npm run test
cd frontend && npm run build
```

GitHub Actions 也会在 `push` / `pull_request` 时自动跑同一套检查。

---

## 目录结构

```text
backend/                 Go 后端
  cmd/server/              服务入口
  internal/ai/             OpenAI 兼容 client + 解析 + 重试
  internal/config/         runtime config / preset 加载
  internal/game/           模板、状态机、AI 行动、历史、回放
  internal/httpapi/        REST API + SSE
frontend/                React + Vite 前端
  src/App.tsx              工作台主入口
  src/components/          座位组件
  src/lib/                 api / sse / types / seatLayout
config/
  app.json                 本地端口与 API 目标
  ai-presets.demo.yaml     可提交的示例配置
  ai-presets.yaml          本地真实配置（gitignored）
.agents/plans/           规划与实现留痕
AGENTS.md                仓库级 agent 工作说明
```

---

## 当前已知限制

- **历史和回放目前仍在内存里**，后端重启后会丢。
- 还没做多人真人联机、自由聊天、警长、警徽流、遗言等扩展规则。
- Replay 已经支持按步骤看，但信息压缩和表现还没完全打磨到参考项目 `holdem` 的程度。
- 使用真实模型会消耗真实 token / 费用，纯 AI 全自动模式尤其要注意。

---

## 相关文件

- 规划与实现留痕：`.agents/plans/2026-06-23-ai-werewolf.md`
- 仓库级工作约束：`AGENTS.md`
