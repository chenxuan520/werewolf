# AI 狼人杀工作台 规划记录

**目标：** 先落一个可演示的 AI 狼人杀工作台首版闭环，能建局、推进、查看历史与回放。
**需求来源：** 2026-06-23 对话需求（参考 `/Users/bytedance/self/holdem`）

## 计划

### 目标

- 做一个形态上参考 `holdem` 的本地优先 AI 狼人杀工作台。
- 同时支持两种模式：纯 AI 观战、1 真人 + 其余 AI。
- 首版重点是“能看效果”的最小闭环，不先追求完整规则编辑器或多人联机。

### 产品形态

- 前端保持工作台结构：Lobby、Live Room、History、Replay。
- 后端负责：建局、状态推进、AI 行动、事件流、回放数据。
- 首个可演示版本优先用脚本化 AI 代理跑通整条链路；后续再扩到真实 OpenAI 兼容模型接入。

### 范围与不做

首版做：

- 纯 AI 观战。
- 1 真人 + 其余 AI。
- 固定 6 / 7 / 8 人板型切换。
- 白天双轮发言（主发言 + 回应）+ 投票。
- 夜晚基础神职链路：狼人、预言家、女巫；7 人加猎人；8 人加守卫。
- 历史记录、赛后回放、实时事件流。

首版不做：

- 多真人联机。
- 自由聊天式讨论。
- 任意身份自定义。
- 警长、警徽流、遗言等扩展规则。
- 为外部模型接入提前铺很重的抽象层。

### 关键决定

- 技术骨架沿用 `holdem`：Go 后端 + React/Vite 前端 + SSE 实时推送。
- 白天流程采用半结构化：每人一轮主发言，再一轮回应，再进入投票。
- 人机局里真人轮到自己时输入发言或技能动作；AI 自动推进到下一个真人决策点。
- 纯 AI 观战模式下，首版先提供手动推进和一键跑完整局两个控制方式。
- 信息展示策略：观战局尽量开放；人机局实时只展示公共信息和真人自己的身份/操作，完整私密日志放到赛后回放。
- 狼人夜晚协作首版采用“各自给出刀口，系统按多数决合并；平票取座位号更小目标”的确定性规则。

### 关键假设

- 6 / 7 / 8 人固定板型分别为：
  - 6 人：2 狼 + 预言家 + 女巫 + 2 民
  - 7 人：在 6 人基础上 + 猎人
  - 8 人：在 7 人基础上 + 守卫
- 首版允许守卫守自己，但不允许连续两晚守同一目标。
- 平票放逐默认“不出人”。
- 猎人死亡后可以立即开枪。
- 演示版 AI 先用脚本化 persona 驱动，保证测试稳定和本地可演示；真实 LLM 接入后再替换同一动作接口。

### 技术上下文

- 仓库当前几乎为空，仅有 `.opencode/`，可以从零搭建最小结构。
- 参考项目 `holdem` 已验证的产品骨架包括：
  - Go 后端状态服务 + HTTP API + SSE
  - React/Vite 多页面工作台
  - 事件流驱动的历史与回放
- 当前首版不假设已有数据库或部署约束，优先本地演示和自测可跑通。

### 结构与落点

- `backend/`
  - `cmd/server/`：服务入口
  - `internal/config/`：运行配置与 AI 预设加载
  - `internal/game/`：狼人杀模板、状态机、回放、AI 决策
  - `internal/httpapi/`：REST API + SSE
- `frontend/`
  - `src/App.tsx`：工作台壳
  - `src/pages/`：Lobby / Room / History / Replay
  - `src/lib/`：api / sse / types
- `config/`
  - `app.json`
  - `ai-presets.yaml`

### 下一步

1. 先写规划记录并建起与 `holdem` 对齐的仓库骨架。
2. 先做“脚本化 AI”版本的首个闭环：Lobby 建局 -> Live Room 推进 -> History / Replay 可看。
3. 自测至少覆盖：Go 测试、前端测试、前端构建。
4. 跑通后再看是否继续接真实 OpenAI 兼容模型。

### 更新日志

- 2026-06-23 19:16：根据 brainstorm 结论创建首版规划记录，确认工作台形态、6/7/8 固定板型、双轮发言、观战/人机双模式，以及“先用脚本化 AI 跑通首个闭环”的实现方向。

## 实现

### 更新日志

- 2026-06-23 19:40：完成首个可演示闭环的最小实现。
  - 做了什么：
    - 搭起 Go 后端基础骨架（`internal/config` / `internal/game` / `internal/httpapi`），实现模板、建局、推进、SSE、历史、回放的最小链路。
    - 搭起 React/Vite 前端工作台，提供 Lobby / Room / History / Replay 四个标签页。
    - 落了脚本化 AI 流程：纯 AI 观战支持“下一步 / 跑完整局”，人机局支持夜间选目标、白天发言、回应推进。
    - 按用户后续纠偏，实际首版采用基础小板子：6=2狼2民预女，7=2狼3民预女，8=3狼3民预女，9=3狼3民预女猎；此前计划里的旧板型假设已被本实现覆盖。
  - 改了哪些文件：
    - `backend/cmd/server/main.go`
    - `backend/internal/config/runtime.go`
    - `backend/internal/config/presets.go`
    - `backend/internal/game/types.go`
    - `backend/internal/game/templates.go`
    - `backend/internal/game/service.go`
    - `backend/internal/game/service_test.go`
    - `backend/internal/httpapi/server.go`
    - `frontend/package.json`
    - `frontend/index.html`
    - `frontend/tsconfig*.json`
    - `frontend/vite.config.ts`
    - `frontend/vitest.config.ts`
    - `frontend/src/App.tsx`
    - `frontend/src/lib/api.ts`
    - `frontend/src/lib/sse.ts`
    - `frontend/src/lib/types.ts`
    - `frontend/src/main.tsx`
    - `frontend/src/styles.css`
    - `frontend/tests/setup.ts`
    - `frontend/tests/app.test.tsx`
    - `config/app.json`
    - `config/ai-presets.yaml`
    - `config/ai-presets.demo.yaml`
    - `.gitignore`
  - 做了哪些自测 / 验证：
    - `cd backend && go test ./...`
    - `cd frontend && npm run test`
    - `cd frontend && npm run build`
    - 启动本地服务后做浏览器 smoke：
      - Lobby 成功建 6 人纯 AI 观战局。
      - 观战局成功跑完整局，并在 History / Replay 中可查看事件流。
      - Lobby 成功建 1 真人 + 5 AI 局，真人夜间刀口与白天发言都能进入下一阶段。
  - 结果如何：
    - 前后端编译、测试、构建全部通过。
    - 本地可访问 `http://127.0.0.1:5174` 查看效果。
  - 风险或阻塞：
    - 记录与回放当前仅驻留内存，后端重启会丢失；还没接 SQLite 持久化。
    - AI 仍是脚本化 persona，不是真实 LLM 调用。
    - 规则仍是 MVP 版本，未覆盖警长、遗言、更多变体板子。

- 2026-06-23 20:29：接入真实 AI 配置并完成真实联调验证，同时把房间主视觉改成更接近 holdem 的环桌座位布局。
  - 做了什么：
    - 把 `werewolf/config/ai-presets.yaml` 改成真实 OpenAI 兼容预设，直接复用参考项目的 endpoint / token / model；同时把 `config/ai-presets.yaml` 加入 `.gitignore`，保留 `config/ai-presets.demo.yaml` 作为可提交模板。
    - 后端新增 `internal/ai/`，把 AI 决策从脚本化 persona 扩展到真实 LLM 调用；支持 tool_call / json_object，且补了 3 次重试，降低模型返回半截 JSON 时的 fallback 概率。
    - 房间页和回放页改成类似 holdem 的环桌主舞台 + 右侧侧栏结构；新增 `WerewolfSeat`、椭圆 seat layout、中心台面文案、可读化事件时间线。
    - 继续保留脚本 fallback，作为真实模型出错时的兜底，但当前最新联调已做到 0 fallback 完成整局。
  - 改了哪些文件：
    - `.gitignore`
    - `config/ai-presets.yaml`
    - `config/ai-presets.demo.yaml`
    - `backend/internal/ai/client.go`
    - `backend/internal/ai/parse.go`
    - `backend/internal/game/service.go`
    - `frontend/src/App.tsx`
    - `frontend/src/styles.css`
    - `frontend/src/components/WerewolfSeat.tsx`
    - `frontend/src/lib/seatLayout.ts`
  - 做了哪些自测 / 验证：
    - `cd backend && go test ./...`
    - `cd frontend && npm run test`
    - `cd frontend && npm run build`
    - 真实 AI 纯观战验收：本地 `classic-6 + glm-5-1 x6` 跑完整局，结果 `finished / 好人阵营 / 71 events / 0 fallback`。
    - 真实 AI 人机验收：本地 `classic-6 + 1 真人 + glm-5-1 x5` 从建局推进到结束，结果 `finished / 好人阵营 / 85 events / 0 fallback`。
    - 额外浏览器 smoke：前端可正常加载真实预设列表，并展示环桌主舞台与侧栏。
  - 结果如何：
    - 真实 AI 已经不是“只接上配置”，而是实际能把一局从建局跑到结束。
    - 房间页不再是简单列表，而是对齐到参考项目的“环桌座位 + 舞台中心 + 侧栏事件流”结构。
  - 风险或阻塞：
    - 当前真实联调优先跑的是 `GLM-5.1`；其它模型虽然能出现在选择列表里，但还没逐个验收。
    - 后端仍未做 SQLite 持久化，重启会丢历史。
    - 规则仍是 MVP；如果继续往正式产品走，还需要补警长/遗言/更完整的回放摘要层。

- 2026-06-23 21:51：补了站点 favicon，把浏览器标签页的 icon 从默认空白改成狼人杀主题图标。
  - 做了什么：
    - 新增 `frontend/public/favicon.svg`，做了一个深蓝夜色底 + 月牙 + 狼头轮廓的 SVG icon。
    - 在 `frontend/index.html` 里接上 `rel="icon"`，并补了 `theme-color`。
  - 改了哪些文件：
    - `frontend/public/favicon.svg`
    - `frontend/index.html`
  - 做了哪些自测 / 验证：
    - `cd frontend && npm run build`
    - 访问 `http://127.0.0.1:5174/favicon.svg`，确认 dev server 能返回新图标资源。
  - 结果如何：
    - favicon 已接入，刷新页面后浏览器标签页会显示新的狼人杀 icon。
  - 风险或阻塞：
    - 当前只做了 SVG favicon；如果后续要兼容更老的浏览器或移动端桌面图标，再补 PNG / apple-touch-icon 即可。

- 2026-06-25 01:22：继续把观战推进、回放步进和房间气泡行为往 holdem 对齐，并补上 gitignore 漏项。
  - 做了什么：
    - 后端补了 `ControlState` 驱动的观战控制语义：`pause / continue / auto_on / semi_auto_on / manual_on / step / stop`，并新增 `/api/games/{id}/control`。
    - spectator 半自动从“每阶段开始前暂停”改成“当天投票结算后暂停”；实际暂停点落在 `vote_resolved / player_eliminated` 之后、进入下一夜之前。
    - 房间页控制条显示语义同步改成 `继续下一天`；手动模式仍用 `下一步`。
    - AI 输入日志改成“当前轮次内完整可见信息”，不是只给最近几条。
    - 发言气泡改成只显示最新一条（实时房间）/ 当前步骤一条（回放），不再把整轮所有发言都挂在牌桌上挡住座位。
    - 牌桌继续区分 `night / day / vote` 三套主色，并把 `frontend/tsconfig.app.tsbuildinfo`、`.opencode/` 等补进 `.gitignore`。
  - 改了哪些文件：
    - `.gitignore`
    - `backend/internal/game/types.go`
    - `backend/internal/game/service.go`
    - `backend/internal/httpapi/server.go`
    - `frontend/src/lib/types.ts`
    - `frontend/src/lib/api.ts`
    - `frontend/src/lib/seatLayout.ts`
    - `frontend/src/components/WerewolfSeat.tsx`
    - `frontend/src/App.tsx`
    - `frontend/src/styles.css`
  - 做了哪些自测 / 验证：
    - `cd backend && go test ./...`
    - `cd frontend && npm run test`
    - `cd frontend && npm run build`
    - 后端接口 smoke：创建 `classic-6 + spectator + semiAutoMode` 真实 AI 局后轮询 `/api/games/{id}`，确认状态停在 `paused / day_vote / running=false`，最后事件包含 `vote_resolved` 和 `player_eliminated`。
    - MCP 视觉检查：Lobby 已显示三档观战推进方式；Room 已显示控制条、最新发言气泡；Replay 已显示步骤控制和步骤列表。
  - 结果如何：
    - spectator 半自动现在按“当天投票结束后停下”工作。
    - 房间气泡不再一口气挡住一桌座位，已经收敛成更接近 holdem 的单气泡显示。
  - 风险或阻塞：
    - 还没把 werewolf 的回放内容压到和 holdem 一样精炼（现在已有步骤控制，但 feed 仍偏事件流）。
    - 仍需继续做一次完整的 MCP 视觉回归，确认所有按钮在 dev server 重启后状态稳定。

- 2026-06-25 10:48：创建 public GitHub 仓库并完成首次推送。
  - 做了什么：
    - 检查当前仓库状态、忽略规则和最近计划记录，确认 `config/ai-presets.yaml` 仍被 `.gitignore` 排除，没有把本地真实 token 一起提交。
    - 提交当前代码为首个 git commit：`feat: bootstrap werewolf workbench`。
    - 使用 `gh repo create chenxuan520/werewolf --public --source=. --remote=origin --push` 创建远端并推送当前分支。
  - 改了哪些文件：
    - `.agents/plans/2026-06-23-ai-werewolf.md`
  - 做了哪些自测 / 验证：
    - `cd backend && go test ./...`
    - `cd frontend && npm run test`
    - `cd frontend && npm run build`
    - `git status --short --ignored`（确认 `config/ai-presets.yaml` 仍是 ignored）
    - `gh repo create chenxuan520/werewolf --public --source=. --remote=origin --push`
  - 结果如何：
    - 远端仓库已创建并推送成功：`https://github.com/chenxuan520/werewolf`
    - 本地 `master` 已跟踪 `origin/master`。
  - 风险或阻塞：
    - 当前公开仓库仍缺 README 等补充说明；如果后续要给外部用户直接使用，还需要再补安装/启动说明。

- 2026-06-25 10:50：补齐公开仓库文档，新增 README 和仓库级 AGENTS 说明。
  - 做了什么：
    - 新增根 `README.md`，补上项目简介、当前规则范围、启动方式、观战/人机模式说明、目录结构、验证命令和当前限制。
    - 新增仓库级 `AGENTS.md`，约束后续 agent 不要提交真实 `ai-presets.yaml`，并明确当前板型、观战半自动语义、验证命令、重启后端注意事项、实现留痕位置。
    - 文档内容统一按当前真实实现落地，而不是沿用计划里早期已经过期的 7/8 人板型假设。
  - 改了哪些文件：
    - `README.md`
    - `AGENTS.md`
    - `.agents/plans/2026-06-23-ai-werewolf.md`
  - 做了哪些自测 / 验证：
    - 复核 `backend/cmd/server/main.go`、`frontend/package.json`、`frontend/vite.config.ts`、`config/app.json`，确认 README / AGENTS 中的启动命令、端口、配置路径与当前代码一致。
    - 复核当前实现日志与实际板型，确认文档里写的是已落地的 `classic-6/7/8/9`，不是早期错误假设。
  - 结果如何：
    - 公开仓库现在具备最基本的使用说明和仓库级 agent 约束，外部读者能按文档启动，后续 agent 也有统一落点可遵循。
  - 风险或阻塞：
    - 当前 README 仍以本地运行说明为主，后续如果要面向陌生用户，还可以继续补截图、API 说明或更细的玩法说明。

## 审查
