# 部署指南

面向运维：Docker Hub 拉取部署、Docker Hub 发布，以及多监听器远程访问。所有内容对照仓库里的 `docker-compose.yml`、`Dockerfile` 与 `config.example.json`。

镜像已发布在 Docker Hub：**[`buffer1705/sk5proxy`](https://hub.docker.com/r/buffer1705/sk5proxy)**，当前提供 `v1.0.0` 与 `latest` 标签，多架构清单覆盖 `linux/amd64` 与 `linux/arm64`。同一标签在拉取时按宿主架构自动匹配，无需手动挑选。

## 1. Docker Hub 部署

### 1.1 前置

- 目标宿主机已安装 Docker 与 Docker Compose 插件。
- 能访问 Docker Hub。`buffer1705/sk5proxy` 为 Public 仓库可匿名拉取；若发布方改为 Private，部署机需先 `docker login`。
- 随仓库取得 `docker-compose.yml` 与 `.env.example`（README 快速开始一节也附有一份完整可用的 `docker-compose.yml`，无仓库时可直接照抄）。

### 1.2 拉取镜像并启动

```bash
cp .env.example .env          # 示例默认 SK5_IMAGE=buffer1705/sk5proxy:v1.0.0
docker compose pull           # 拉取已发布的多架构镜像
docker compose up -d
```

`docker-compose.yml` 关键点：

- `image: ${SK5_IMAGE:-buffer1705/sk5proxy:v1.0.0}`：默认即 Docker Hub 已发布镜像；`.env` 里的 `SK5_IMAGE` 可覆盖为别的已发布版本。
- `pull_policy: missing`：本地缺失该镜像时联网拉取，已有则直接用（因此上面显式 `docker compose pull` 拉取一次更直观）。
- 没有 `build:` 段：不会触发源码构建，也不会有 Go 模块下载。

> 升级已有部署时不要覆盖 `.env`，只改其中 `SK5_IMAGE` 一行再 `docker compose pull && docker compose up -d`，详见第 2.3 节。生产建议固定到具体版本号，不要用会随发布漂移的 `latest`。

### 1.3 验证

```bash
docker compose ps
curl http://127.0.0.1:8081/healthz          # 期望返回 ok
curl http://127.0.0.1:8081/api/config       # 期望返回公共配置 JSON
```

`healthz` 由容器内 healthcheck 每 10s 探测一次（`wget -q -O - http://127.0.0.1:8081/healthz`）。`docker compose ps` 的 `STATUS` 列出现 `healthy` 只表示管理 HTTP 端点可响应。

> 该 healthcheck 只探测本服务自身的 `/healthz`，**不**对上游代理做任何健康检查或探活。

## 2. Docker Hub 发布

### 2.1 当前发布状态

`v1.0.0` 已通过 CI 成功发布到 **[`buffer1705/sk5proxy`](https://hub.docker.com/r/buffer1705/sk5proxy)**，多架构清单覆盖 `linux/amd64` 与 `linux/arm64`，同时打了 `latest`。对应构建运行见 GitHub Actions：`https://github.com/2Wgmqra4ZuDMex/sk5proxy/actions/runs/34645859754`。部署机拉取 `buffer1705/sk5proxy:v1.0.0` 后 `/healthz` 已实测返回 `ok`。

以下 2.2–2.4 记录发布机制本身，供后续版本发布时参考。

### 2.2 工作流与触发

工作流 `.github/workflows/docker-publish.yml` 使用现有 `Dockerfile`，分别构建 `linux/amd64` 和 `linux/arm64`，推送架构临时标签后创建多架构清单。它只在以下情况运行，不响应 Pull Request，因此 PR 不会取得发布凭据：

- 推送符合 `v*` 的 Git tag；工作流会进一步要求严格版本 `vMAJOR.MINOR.PATCH`，可带预发布后缀（例如 `v1.0.0-rc.1`）。
- GitHub Actions 页面手动运行，并输入同样格式的 `version`。

### 2.3 用户必须提供的输入

1. 在 Docker Hub 注册/登录账号。
2. 在该账号下创建名为 **`sk5proxy`** 的仓库；选择 Public 还是 Private 由你决定。Private 镜像的部署机器还需先 `docker login`。
3. 在 Docker Hub 创建具有该仓库 Read/Write 权限的 Access Token，不要使用账号密码。
4. 在 GitHub 仓库 `Settings → Secrets and variables → Actions` 添加两个 **Repository secrets**：
   - `DOCKERHUB_USERNAME`：Docker Hub 用户名；工作流也用它组成 `<用户名>/sk5proxy` 镜像名（当前发布方为 `buffer1705`）。
   - `DOCKERHUB_TOKEN`：上一步的 Access Token。

当前不需要普通 GitHub Actions Variable。不要把用户名、Token 写入仓库文件或 `.env`；用户名虽然不敏感，也统一通过 Secret 输入以满足镜像名配置。未设置这两个 Secret 前，工作流会在登录步骤失败，不会发布任何镜像。

### 2.4 标签规则与后续发布

```bash
git tag v1.0.1
git push origin v1.0.1
```

稳定版生成 `v1.0.1`、`1.0.1`、`1.0` 和 `latest`。预发布版生成版本标签但**不会**更新 `latest`，因此任意开发提交不会被误标为最新版。手动运行仍要求合法 SemVer；它适合重新发布明确版本，不接受 `dev`、分支名或空值。**不要对已发布的 `v1.0.0` 重新打同一 tag 覆盖已发布镜像**；发新版请用新的版本号。

部署远程镜像时复制 `.env.example`（其默认已指向 `buffer1705/sk5proxy:v1.0.0`），需要别的版本时改为：

```dotenv
SK5_IMAGE=buffer1705/sk5proxy:v1.0.0
```

随后运行 `docker compose pull && docker compose up -d`。Public 仓库可匿名拉取；Private 仓库必须先登录 Docker Hub。生产环境建议固定到具体版本号，不要依赖会随发布漂移的 `latest`。

## 3. 端口与发布

| 端口 | 用途 | compose 默认发布地址 |
| --- | --- | --- |
| `8081` | Web 面板 / `/api` / `/healthz` | `127.0.0.1` |
| `1080` | 迁移用默认 SOCKS5 | `127.0.0.1` |
| `8080` | 迁移用默认 HTTP | `127.0.0.1` |
| `10080-10180` | 动态监听器端口范围 | `${SK5_PROXY_BIND_IP:-127.0.0.1}` |

`docker-compose.yml` 的 `ports` 段：

```yaml
ports:
  - "127.0.0.1:1080:1080"
  - "127.0.0.1:8080:8080"
  - "127.0.0.1:8081:8081"
  - "${SK5_PROXY_BIND_IP:-127.0.0.1}:${SK5_PROXY_PORT_RANGE:-10080-10180}:${SK5_PROXY_PORT_RANGE:-10080-10180}"
```

要点：

- 只有动态范围那一行的发布地址与范围可通过 `.env` 改（`SK5_PROXY_BIND_IP`、`SK5_PROXY_PORT_RANGE`）。8081/1080/8080 三行在 compose 里写死回环。
- 容器内进程按环境变量把默认监听器绑到 `0.0.0.0`（`SK5_SOCKS_ADDR=0.0.0.0:1080` 等），那是**容器内**监听。对宿主机之外能否连上，取决于上面 `ports` 发布到哪个地址。**Docker 不会自动发布**监听器端口。
- Web 面板里新建的监听器，其端口必须落在启动时已发布的 `SK5_PROXY_PORT_RANGE` 内。改了范围要重建容器（再跑一次 `up -d`）才生效。`Dockerfile` 里的 `EXPOSE 1080 8080 8081 10080-10180/tcp` 只是元数据，不发布端口。

## 4. `.env` 配置

在 compose 文件同目录放 `.env`（`cp .env.example .env` 即得下面这份，默认已指向已发布镜像）：

```dotenv
# 要用的镜像；默认拉 Docker Hub 已发布版本
SK5_IMAGE=buffer1705/sk5proxy:v1.0.0
# 动态监听器范围对外发布到哪个宿主机地址
SK5_PROXY_BIND_IP=127.0.0.1
# 动态监听器端口范围（宿主发布范围要与 Web 里创建的端口对得上）
SK5_PROXY_PORT_RANGE=10080-10180
```

- 只想本机用：保持 `127.0.0.1`。
- 想给局域网用：改成宿主机的 LAN 网卡地址，见下一节。
- 用上面 README 里那份完整 YAML、不建 `.env` 时：compose 内建默认已是 `buffer1705/sk5proxy:v1.0.0` 且绑定回环，可直接启动；仅在需要改发布地址/端口范围时才补一份 `.env`。

**升级已有部署时不要用 `cp .env.example .env` 覆盖**你现有的 `.env`，那会连同 `SK5_PROXY_BIND_IP`、端口范围等网络设置一起被重置。只手动改现有 `.env` 里的 `SK5_IMAGE` 一行，其余保持不动，然后 `docker compose pull && docker compose up -d`。同时保持同一项目目录 / compose 项目名与命名卷 `sk5proxy-data`，切勿 `docker compose down -v`，否则会丢配置。

## 5. 多监听器与远程访问

### 5.1 新建监听器

通过 Web 面板或 `PUT /api/config/listeners/{id}` 创建。监听器字段：`name`、`type`（`socks5` 或 `http`）、`address`（`host:port`）、`upstreamId`、`enabled`。容器内地址填 `0.0.0.0:<端口>`，端口须在已发布范围内。多条监听器各自独立绑定、独立选上游。

### 5.2 对局域网开放代理

1. `.env` 把范围发布到 LAN 地址：

   ```dotenv
   SK5_PROXY_BIND_IP=192.0.2.10
   SK5_PROXY_PORT_RANGE=10080-10180
   ```

2. `up -d` 重建后，同网段客户端连 `192.0.2.10:10080` 这类地址。
3. 客户端配置里的“代理服务器地址”写宿主机 LAN IP（`192.0.2.10`），不是 `0.0.0.0`。`0.0.0.0` 只是容器内 bind 全网卡的写法，不是客户端能连的目标地址。

### 5.3 管理面板远程访问

面板端口 8081 在 compose 里写死 `127.0.0.1`。如确需从别的机器打开：

- **推荐：SSH 隧道**（面板无认证，隧道最稳妥）：

  ```bash
  ssh -L 8081:127.0.0.1:8081 user@192.0.2.10
  # 本机浏览器访问 http://127.0.0.1:8081
  ```

- 或**显式改映射**到 LAN IP（自担风险，务必配防火墙）：把 compose 里
  `"127.0.0.1:8081:8081"` 改成 `"192.0.2.10:8081:8081"` 再 `up -d`。

## 6. 安全

- **无入站认证**：下游代理端口和管理 API 都不认证。切勿把 8081 发布到公网。
- 把代理开放给其他机器时，用主机防火墙 / 云安全组只放行可信源 IP，优先绑专用 LAN/VPN 网卡。
- **Docker + UFW 陷阱**：Docker 发布端口时直接写 iptables 的 `DOCKER` 链，通常**绕过** UFW 的默认策略。也就是说，即使 `ufw deny` 了某端口，Docker 发布的端口仍可能对外可达。对策：
  - 优先用 `.env` 的 `SK5_PROXY_BIND_IP` 把发布地址收窄到回环或特定 LAN IP，而不是 `0.0.0.0`；
  - 或在云安全组 / 网络 ACL 层面做限制（这层在宿主机之外，不受 Docker iptables 影响）；
  - 或按 Docker 官方文档针对 `DOCKER-USER` 链配置规则。
- 不要默认用无限制 `0.0.0.0` 发布。

## 7. 数据卷与生命周期

- 配置持久化在命名卷 `sk5proxy-data`，挂到容器 `/data`，实际文件是 `/data/config.json`。
- 卷名由 compose 项目名派生（默认取所在目录名）。**要复用同一份配置，就在同一目录用同一 compose 项目名启动。** 换目录或换 `-p` 项目名会得到另一个空卷。
- `docker compose down` 保留卷。**永远不要 `docker compose down -v`**，`-v` 会删掉命名卷，连同 `config.json` 一起丢失。
- 常规操作：
  - 停止：`docker compose down`
  - 重启：`docker compose up -d`
  - 看日志：`docker compose logs -f`
