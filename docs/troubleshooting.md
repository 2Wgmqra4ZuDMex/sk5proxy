# 故障排查

对照仓库里的 `docker-compose.yml`、`Dockerfile`、`config.example.json` 与 `internal/` 实现。

## 不需要构建、不需要下载 Go 模块

Docker Hub 部署直接拉已发布的多架构镜像，**不做源码构建，也不下载任何 Go 模块**，因此不会遇到 `go mod download` 超时之类的构建期网络问题。默认 `docker-compose.yml` 没有 `build:` 段，只拉镜像：

```bash
# .env 里 SK5_IMAGE=buffer1705/sk5proxy:v1.0.0（cp .env.example .env 即是此默认）
docker compose pull
docker compose up -d
```

若 `docker compose pull` 本身报网络超时，是宿主机到 Docker Hub 的连通性问题：确认能访问 Docker Hub、必要时配置镜像加速或代理；Private 仓库还需先 `docker login`。

## 端口绑定冲突

**现象**：`up -d` 报 `bind: address already in use` / `port is already allocated`；或 Web 里启用监听器返回 `409`。

**分两种情况**：

1. **宿主机端口被占**（8081/1080/8080 或动态范围）：说明宿主机已有进程或别的容器占了该端口。

   ```bash
   ss -ltnp | grep -E ':(8081|1080|8080|10080)'   # 看谁占了
   docker compose ps                               # 看是否重复启动
   ```

   腾出端口，或改 compose 映射 / `.env` 范围后重建。

2. **容器内监听器地址冲突**：两个 `enabled` 的监听器绑同一 `address` 会被配置校验直接拒绝（见 `internal/config` 的重复地址校验），API 返回 `400`；若地址不冲突但底层 bind 失败则返回 `409`。给每个监听器分配范围内且互不相同的端口。

**动态范围外的端口连不上**：监听器端口必须落在启动时发布的 `SK5_PROXY_PORT_RANGE` 内。改了范围要重新 `up -d` 重建容器，`EXPOSE` 不发布端口。

## 架构不匹配：amd64 与 ARM

**现象**：启动后容器立刻退出，报 `exec format error` 或平台不匹配警告。

**原因**：容器镜像架构与宿主机不符。

**通常不该发生**：Docker Hub 上的 `buffer1705/sk5proxy` 是覆盖 `linux/amd64` 与 `linux/arm64` 的多架构清单，`docker compose pull` 会按宿主架构自动挑选，无需手动区分。

**确认宿主机架构**：

```bash
uname -m    # x86_64 = amd64；aarch64/arm64 = ARM
```

若确实报错，多半是被固定到了单架构标签或手动搬运了不匹配的镜像。改回默认的多架构标签重拉即可：

```bash
docker pull buffer1705/sk5proxy:v1.0.0   # 多架构清单，自动匹配
docker compose up -d
```

需要发布尚未覆盖的架构时，通过 CI（`.github/workflows/docker-publish.yml`，构建 amd64 与 arm64）打新版本，见 `docs/deployment.md`。

## 查看日志与健康状态

```bash
# 实时日志
docker compose logs -f

# 容器与健康状态（STATUS 列会显示 healthy / unhealthy）
docker compose ps

# 直接探管理 HTTP 端点
curl -i http://127.0.0.1:8081/healthz
```

healthcheck 定义在 compose 里：每 10s 跑一次 `wget -q -O - http://127.0.0.1:8081/healthz`，超时 3s，连续 3 次失败标记 `unhealthy`，启动宽限 3s。

**该 healthcheck 只探本服务自身**，不代表任何上游代理可用；本服务不对上游做健康检查或探活。

**`unhealthy` 常见排查**：

- `logs` 里看是否有配置校验错误导致启动失败（如 `activeId` 指向不存在的 upstream、监听器地址重复、地址不是 `host:port`）。
- 确认 8081 在容器内正常监听（`SK5_WEB_ADDR` 默认 `0.0.0.0:8081`）。

## 远程连不上代理

- 客户端要连**宿主机**地址，不是 `0.0.0.0`。`0.0.0.0` 只是容器内 bind 全网卡的写法。
- 检查 `.env` 的 `SK5_PROXY_BIND_IP` 是否发布到了客户端能到达的地址（回环只有本机能连）。
- 检查监听器端口是否在已发布范围内、且该监听器 `enabled`。
- 检查主机防火墙 / 云安全组。注意 **Docker 发布端口常绕过 UFW**（Docker 直接改 iptables `DOCKER` 链），必要时在云安全组或 `DOCKER-USER` 链层面放行 / 限制，详见 `docs/deployment.md` 安全小节。

## 配置相关

- **`activeId` 报校验失败**：`activeId` 非空时必须指向已存在的 upstream。新配置直接留空 `activeId`；路由只看各监听器的 `upstreamId`。
- **上游只填了用户名或只填了密码**：会被拒绝。用户名/密码要么都填、要么都空。
- **改坏了 `config.json`**：配置在命名卷 `sk5proxy-data` 里。不要用 `docker compose down -v`（会删卷丢配置）。需要重置时可单独删该卷后重启，服务会生成默认配置（见 `internal/app` 的初始化/迁移逻辑）。
