# 故障排查

对照仓库里的 `docker-compose.offline.yml`、`docker-compose.yml`、`Dockerfile`、`config.example.json` 与 `internal/` 实现。

## 构建时 Go 模块下载超时（离线环境）

**现象**：`docker compose -f docker-compose.build.yml up -d --build` 卡在 `go mod download` 或类似步骤，报网络超时。

**原因**：显式开发文件 `docker-compose.build.yml` 带 `build:`，构建期要联网拉 Go 模块（`Dockerfile` 里的 `RUN go mod download`）。离线机器上必然失败。

**联网机器**：直接拉 Docker Hub 已发布的多架构镜像即可，完全不构建：

```bash
# .env 里 SK5_IMAGE=buffer1705/sk5proxy:v1.0.0（cp .env.example .env 即是此默认）
docker compose pull
docker compose up -d
```

**离线绕过**：不能联网时改用 `docker load` 的本地归档镜像：

```bash
cd dist
sha256sum -c sk5proxy-v1.0.0-linux-amd64.tar.gz.sha256
docker load -i sk5proxy-v1.0.0-linux-amd64.tar.gz
cd ..
# .env 里 SK5_IMAGE=sk5proxy:offline
docker compose up -d
```

离线镜像已经把二进制打进去了，启动路径完全不触网、不下载任何模块。归档由发布方单独提供，见 `docs/deployment.md`。

若确实要在联网机器上构建，再把镜像导出搬到离线机：

```bash
scripts/build-image.sh v1.0.0
# 将 dist/ 中归档和 .sha256 一起拷到离线机
```

## 端口绑定冲突

**现象**：`up -d` 报 `bind: address already in use` / `port is already allocated`；或 Web 里启用监听器返回 `409`。

**分两种情况**：

1. **宿主机端口被占**（8081/1080/8080 或动态范围）：说明宿主机已有进程或别的容器占了该端口。

   ```bash
   ss -ltnp | grep -E ':(8081|1080|8080|10080)'   # 看谁占了
   docker compose -f docker-compose.offline.yml ps # 看是否重复启动
   ```

   腾出端口，或改 compose 映射 / `.env` 范围后重建。

2. **容器内监听器地址冲突**：两个 `enabled` 的监听器绑同一 `address` 会被配置校验直接拒绝（见 `internal/config` 的重复地址校验），API 返回 `400`；若地址不冲突但底层 bind 失败则返回 `409`。给每个监听器分配范围内且互不相同的端口。

**动态范围外的端口连不上**：监听器端口必须落在启动时发布的 `SK5_PROXY_PORT_RANGE` 内。改了范围要重新 `up -d` 重建容器，`EXPOSE` 不发布端口。

## 架构不匹配：amd64 与 ARM

**现象**：`docker load` 或启动后容器立刻退出，报 `exec format error` 或平台不匹配警告。

**原因**：归档文件名中的架构必须匹配宿主机；`linux-amd64` 镜像无法在 ARM（如 Apple Silicon、树莓派、ARM 云主机）原生运行。

**确认宿主机架构**：

```bash
uname -m    # x86_64 = amd64；aarch64/arm64 = ARM
```

**ARM 机器的做法**：离线 amd64 归档不适用，需在 ARM 机器上自行构建（`Dockerfile` 基于 `golang:1.23.6-alpine`，`CGO_ENABLED=0`，可原生构建对应架构）：

```bash
docker compose -f docker-compose.build.yml up -d --build
```

或在别处用 buildx 跨架构构建 arm64 镜像再 `docker save` 搬过去。amd64 归档不要硬塞给 ARM 宿主。

## 查看日志与健康状态

```bash
# 实时日志
docker compose -f docker-compose.offline.yml logs -f

# 容器与健康状态（STATUS 列会显示 healthy / unhealthy）
docker compose -f docker-compose.offline.yml ps

# 直接探管理 HTTP 端点
curl -i http://127.0.0.1:8081/healthz
```

healthcheck 定义在两份 compose 里：每 10s 跑一次 `wget -q -O - http://127.0.0.1:8081/healthz`，超时 3s，连续 3 次失败标记 `unhealthy`，启动宽限 3s。

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
