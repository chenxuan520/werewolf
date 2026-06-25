# Werewolf Repo Notes

## 项目结构

- `frontend/`: React + Vite 前端工作台
- `backend/`: Go 后端状态机与 HTTP API
- `config/app.json`: 本地运行端口与 API 代理目标
- `config/ai-presets.demo.yaml`: 可提交的 AI 配置模板
- `config/ai-presets.yaml`: 本机真实 AI 配置，默认不提交
- `.agents/plans/2026-06-23-ai-werewolf.md`: 本仓库的规划与实现留痕

## 高优先级约束

- **不要提交** `config/ai-presets.yaml`，它包含本机真实 endpoint / token / model。
- 新环境先复制 demo：
  - `cp config/ai-presets.demo.yaml config/ai-presets.yaml`
- 当前产品边界以 `.agents/plans/2026-06-23-ai-werewolf.md` 为准；实现记录只追加到 `## 实现 -> ### 更新日志`。
- 当前实际板型是：
  - `classic-6`：2 狼 + 2 民 + 预言家 + 女巫
  - `classic-7`：2 狼 + 3 民 + 预言家 + 女巫
  - `classic-8`：3 狼 + 3 民 + 预言家 + 女巫
  - `classic-9`：3 狼 + 3 民 + 预言家 + 女巫 + 猎人
- 首版仍**不做**多人真人联机、自由聊天、警长、警徽流、遗言、任意规则编辑器。
- 纯 AI 观战默认应保持 **半自动模式**：每天投票结算后停下，等待用户继续下一天。
- 当前历史 / 回放还是**内存态**；不要把它描述成已经持久化，也不要默认它能跨后端重启保留。

## 修改 AI / 自动推进 / 回放 时要特别检查

- 是否会显著增加 token 消耗
- 是否可能导致自动推进死循环
- 是否破坏 spectator 的暂停语义
- 是否破坏 replay 与实时事件的一致性
- UI 变更后是否真的做过浏览器 smoke，而不只是静态阅读

## 常用验证

- 后端测试：`cd backend && go test ./...`
- 前端测试：`cd frontend && npm run test`
- 前端构建：`cd frontend && npm run build`

如果改了房间交互、气泡、控制条、回放步进，除了测试/构建，最好再做一次浏览器检查。

## 启动与重启

- 后端：`cd backend && go run ./cmd/server`
- 前端：`cd frontend && npm install && npm run dev`

改了 `config/ai-presets.yaml` 后，后端要重启才会重新加载 preset。

如果要杀旧后端，只杀监听 `18131` 的进程：

- ✅ `lsof -ti:18131 -sTCP:LISTEN | xargs kill`
- ❌ `lsof -ti:18131 | xargs kill`

不带 `-sTCP:LISTEN` 会把连着这个端口的别的进程也列出来，容易把前端或调试连接一起杀掉。

## AI 配置说明

- 默认从 `config/ai-presets.yaml` 读取 preset。
- 可以临时用 `WEREWOLF_PRESETS_PATH=/path/to/file.yaml` 覆盖加载路径做 smoke。
- `config/ai-presets.demo.yaml` 只放占位示例；真实 token 不要写进 demo、README、计划文档、commit message。

## 留痕要求

- 重要实现完成后，把做了什么、验证了什么、结果如何、剩余风险写回 `.agents/plans/2026-06-23-ai-werewolf.md`。
- 如果只是补文档，也要回写到 `## 实现`，避免后续上下文断层。
