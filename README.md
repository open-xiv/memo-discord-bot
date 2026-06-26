## memo-discord-bot

SuMemo 社区 Discord 机器人：斜杠命令做角色绑定 / 同步 / 查询，附带一个 HTTP 端供探针、指标与 webhook 接收。

### Run

```bash
cp .env.example .env        # 至少 DISCORD_BOT_TOKEN / DATABASE_URL / REDIS_URL
go run .
docker compose up -d        # 起本地 pg / redis
```

### Config

| env | required | what it tunes |
|---|---|---|
| `DISCORD_BOT_TOKEN` | ✅ | Discord 网关鉴权；`/status/ready` 关键依赖 |
| `DATABASE_URL` | ✅ | Postgres DSN |
| `REDIS_URL` | ✅ | Redis URL |
| `BOT_WEBHOOK_TOKEN` | ✅* | `/webhook/gha` 的 `Authorization: Bearer ...` |
| `GITHUB_WEBHOOK_SECRET` | ✅* | `/webhook/github` 的 `X-Hub-Signature-256` HMAC |
| `LOG_LEVEL` / `MEMO_ENV` | | 同其他服务 |

完整 env 见 [`.env.example`](.env.example)。

### 斜杠命令

| cmd | 作用 |
|---|---|
| `/bind name server` | 绑定游戏角色（每用户最多 3 个） |
| `/unbind` | 解绑 |
| `/list` | 列出已绑定角色 |
| `/logs` | 触发 FFLogs 同步 |
| `/hide` | 设置已绑定角色的隐私（公开 / 不上榜 / 隐藏，可多选） |
| `/set-hide` | 管理：设置任意角色的隐私等级（公开 / 不上榜 / 隐藏） |

### Endpoints

| route | response | semantics |
|---|---|---|
| `GET /status/live` | 200 | 进程存活 |
| `GET /status/ready` | 200 / 503 | pg + redis + discord-gateway，见 [observability.md](https://github.com/open-xiv/memo-docs/blob/main/standards/observability.md) |
| `GET /status` | memo 标准 body | 人 / 面板可读 |
| `GET /metrics` | prometheus | |
| `POST /webhook/gha` | — | GHA 部署状态推送，Bearer 鉴权 |
| `POST /webhook/github` | — | GitHub 仓库事件，HMAC 鉴权 |
