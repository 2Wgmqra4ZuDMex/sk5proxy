# 部署指南

面向运维：预构建镜像、离线 Docker 部署、源码构建、Docker Hub 发布，以及多监听器远程访问。所有内容对照仓库里的 Compose 文件、`Dockerfile` 与 `config.example.json`。

## 1. 离线 Docker 部署（首选）

适用于目标机器无法联网、或不想在本机构建的场景。

### 1.1 前置

- 目标宿主机已安装 Docker 与 Docker Compose 插件。
- 拿到与版本、宿主架构对应的归档，例如 `sk5proxy-v1.2.3-linux-amd64.tar.gz`，以及同名 `.sha256`。

  归档由发布方**单独提供**，不随 Git 提交。文件名中的 `amd64` 对应 x86_64，`arm64` 对应 aarch64/Apple Silicon；必须选对架构。

- 仓库里的 `docker-compose.offline.yml`（用来启动已载入的镜像，无需源码构建）。

### 1.2 载入镜像并启动

```bash
# 先验证传输完整性，再载入（在 dist 目录执行）
sha256sum -c sk5proxy-v1.2.3-linux-amd64.tar.gz.sha256
docker load -i sk5proxy-v1.2.3-linux-amd64.tar.gz

# 确认镜像在本地
docker image ls sk5proxy

# 默认 Compose 不构建；本地已有 sk5proxy:offline 时直接启动
docker compose up -d
```

默认 `docker-compose.yml` 关键点：

- `image: ${SK5_IMAGE:-sk5proxy:offline}`：`.env` 可切换到 Docker Hub 的版本化镜像。
- `pull_policy: missing`：本地已有归档镜像时不联网；本地缺失且配置了远程镜像时才拉取。
- 没有 `build:` 段：不会触发源码构建，因此也不会有 Go 模块下载。

为兼容已有离线部署，`docker-compose.offline.yml` 继续保留相同服务名、卷和端口，并使用 `pull_policy: never`。严格禁止联网时可继续执行 `docker compose -f docker-compose.offline.yml up -d`。

### 1.3 验证

```bash
docker compose ps
curl http://127.0.0.1:8081/healthz          # 期望返回 ok
curl http://127.0.0.1:8081/api/config       # 期望返回公共配置 JSON
```

`healthz` 由容器内 healthcheck 每 10s 探测一次（`wget -q -O - http://127.0.0.1:8081/healthz`）。`docker compose ps` 的 `STATUS` 列出现 `healthy` 只表示管理 HTTP 端点可响应。

> 该 healthcheck 只探测本服务自身的 `/healthz`，**不**对上游代理做任何健康检查或探活。

### 1.4 生成并保留本地归档

联网构建机执行：

```bash
scripts/build-image.sh v1.2.3
```

脚本优先使用 buildx，旧环境没有 buildx 时回退到本机架构的传统 `docker build`。它会构建 `sk5proxy:v1.2.3`、同时标记 `sk5proxy:offline`，然后生成：

- `dist/sk5proxy-v1.2.3-linux-<架构>.tar.gz`
- `dist/sk5proxy-v1.2.3-linux-<架构>.tar.gz.sha256`

脚本启用 `pipefail`，写入期间使用临时文件，而且拒绝覆盖同版本已有产物。`dist/`、归档和真实 `.env` 都被 Git 忽略，不会误提交。

## 2. 从源码构建部署（显式开发入口）

目标机器能联网、或你想自己出镜像时：

```bash
docker compose -f docker-compose.build.yml up -d --build
```

只有 `docker-compose.build.yml` 带 `build:` 段，会用仓库根目录的 `Dockerfile` 构建 `sk5proxy:local`。构建期需要联网下载 Go 模块（`go mod download`）。默认 `docker-compose.yml` 不会隐式构建。

## 3. Docker Hub 自动发布

工作流 `.github/workflows/docker-publish.yml` 使用现有 `Dockerfile`，分别构建 `linux/amd64` 和 `linux/arm64`，推送架构临时标签后创建多架构清单。它只在以下情况运行，不响应 Pull Request，因此 PR 不会取得发布凭据：

- 推送符合 `v*` 的 Git tag；工作流会进一步要求严格版本 `vMAJOR.MINOR.PATCH`，可带预发布后缀（例如 `v1.2.3-rc.1`）。
- GitHub Actions 页面手动运行，并输入同样格式的 `version`。

### 3.1 用户必须提供的输入

1. 在 Docker Hub 注册/登录账号。
2. 在该账号下创建名为 **`sk5proxy`** 的仓库；选择 Public 还是 Private 由你决定。Private 镜像的部署机器还需先 `docker login`。
3. 在 Docker Hub 创建具有该仓库 Read/Write 权限的 Access Token，不要使用账号密码。
4. 在 GitHub 仓库 `Settings → Secrets and variables → Actions` 添加两个 **Repository secrets**：
   - `DOCKERHUB_USERNAME`：Docker Hub 用户名；工作流也用它组成 `<用户名>/sk5proxy` 镜像名。
   - `DOCKERHUB_TOKEN`：上一步的 Access Token。

当前不需要普通 GitHub Actions Variable。不要把用户名、Token 写入仓库文件或 `.env`；用户名虽然不敏感，也统一通过 Secret 输入以满足镜像名配置。未设置这两个 Secret 前，工作流会在登录步骤失败，不会发布任何镜像。

### 3.2 标签规则与触发

```bash
git tag v1.2.3
git push origin v1.2.3
```

稳定版生成 `v1.2.3`、`1.2.3`、`1.2` 和 `latest`。预发布版生成版本标签但**不会**更新 `latest`，因此任意开发提交不会被误标为最新版。手动运行仍要求合法 SemVer；它适合重新发布明确版本，不接受 `dev`、分支名或空值。

部署远程镜像时复制 `.env.example` 并设置：

```dotenv
SK5_IMAGE=<dockerhub-username>/sk5proxy:v1.2.3
```

随后运行 `docker compose up -d`。Public 仓库可匿名拉取；Private 仓库必须先登录 Docker Hub。

## 4. 端口与发布

| 端口 | 用途 | compose 默认发布地址 |
| --- | --- | --- |
| `8081` | Web 面板 / `/api` / `/healthz` | `127.0.0.1` |
| `1080` | 迁移用默认 SOCKS5 | `127.0.0.1` |
| `8080` | 迁移用默认 HTTP | `127.0.0.1` |
| `10080-10180` | 动态监听器端口范围 | `${SK5_PROXY_BIND_IP:-127.0.0.1}` |

两份 compose 的 `ports` 段一致：

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

## 5. `.env` 配置

在 compose 文件同目录放 `.env`：

```dotenv
# 动态监听器范围对外发布到哪个宿主机地址
SK5_IMAGE=sk5proxy:offline
SK5_PROXY_BIND_IP=127.0.0.1
# 动态监听器端口范围（宿主发布范围要与 Web 里创建的端口对得上）
SK5_PROXY_PORT_RANGE=10080-10180
```

- 只想本机用：保持 `127.0.0.1`。
- 想给局域网用：改成宿主机的 LAN 网卡地址，见下一节。

## 6. 多监听器与远程访问

### 6.1 新建监听器

通过 Web 面板或 `PUT /api/config/listeners/{id}` 创建。监听器字段：`name`、`type`（`socks5` 或 `http`）、`address`（`host:port`）、`upstreamId`、`enabled`。容器内地址填 `0.0.0.0:<端口>`，端口须在已发布范围内。多条监听器各自独立绑定、独立选上游。

### 6.2 对局域网开放代理

1. `.env` 把范围发布到 LAN 地址：

   ```dotenv
   SK5_PROXY_BIND_IP=192.0.2.10
   SK5_PROXY_PORT_RANGE=10080-10180
   ```

2. `up -d` 重建后，同网段客户端连 `192.0.2.10:10080` 这类地址。
3. 客户端配置里的“代理服务器地址”写宿主机 LAN IP（`192.0.2.10`），不是 `0.0.0.0`。`0.0.0.0` 只是容器内 bind 全网卡的写法，不是客户端能连的目标地址。

### 6.3 管理面板远程访问

面板端口 8081 在 compose 里写死 `127.0.0.1`。如确需从别的机器打开：

- **推荐：SSH 隧道**（面板无认证，隧道最稳妥）：

  ```bash
  ssh -L 8081:127.0.0.1:8081 user@192.0.2.10
  # 本机浏览器访问 http://127.0.0.1:8081
  ```

- 或**显式改映射**到 LAN IP（自担风险，务必配防火墙）：把 compose 里
  `"127.0.0.1:8081:8081"` 改成 `"192.0.2.10:8081:8081"` 再 `up -d`。

## 7. 安全

- **无入站认证**：下游代理端口和管理 API 都不认证。切勿把 8081 发布到公网。
- 把代理开放给其他机器时，用主机防火墙 / 云安全组只放行可信源 IP，优先绑专用 LAN/VPN 网卡。
- **Docker + UFW 陷阱**：Docker 发布端口时直接写 iptables 的 `DOCKER` 链，通常**绕过** UFW 的默认策略。也就是说，即使 `ufw deny` 了某端口，Docker 发布的端口仍可能对外可达。对策：
  - 优先用 `.env` 的 `SK5_PROXY_BIND_IP` 把发布地址收窄到回环或特定 LAN IP，而不是 `0.0.0.0`；
  - 或在云安全组 / 网络 ACL 层面做限制（这层在宿主机之外，不受 Docker iptables 影响）；
  - 或按 Docker 官方文档针对 `DOCKER-USER` 链配置规则。
- 不要默认用无限制 `0.0.0.0` 发布。

## 8. 数据卷与生命周期

- 配置持久化在命名卷 `sk5proxy-data`，挂到容器 `/data`，实际文件是 `/data/config.json`。
- 卷名由 compose 项目名派生（默认取所在目录名）。**要复用同一份配置，就在同一目录用同一 compose 项目名启动。** 换目录或换 `-p` 项目名会得到另一个空卷。
- `docker compose down` 保留卷。**永远不要 `docker compose down -v`**，`-v` 会删掉命名卷，连同 `config.json` 一起丢失。
- 常规操作：
  - 停止：`docker compose -f docker-compose.offline.yml down`
  - 重启：`docker compose -f docker-compose.offline.yml up -d`
  - 看日志：`docker compose -f docker-compose.offline.yml logs -f`
