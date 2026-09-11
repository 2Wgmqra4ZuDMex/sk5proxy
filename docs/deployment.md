# 部署指南

面向运维：离线 Docker 部署，以及多监听器远程访问的配置方式。所有内容对照仓库里的 `docker-compose.offline.yml`、`docker-compose.yml`、`Dockerfile` 与 `config.example.json`。

## 1. 离线 Docker 部署（首选）

适用于目标机器无法联网、或不想在本机构建的场景。

### 1.1 前置

- 目标宿主机已安装 Docker 与 Docker Compose 插件。
- 拿到离线镜像归档 `sk5proxy-linux-amd64.tar.gz`。

  该归档由发布方**单独提供**，不随 Git 仓库分发，除非某个 Release 明确附带了它。仓库里出现同名文件不代表它就是官方发布物，也不要去不存在的 Release 页面找它。归档是 amd64（x86_64）镜像，只能在 amd64 宿主机上运行。

- 仓库里的 `docker-compose.offline.yml`（用来启动已载入的镜像，无需源码构建）。

### 1.2 载入镜像并启动

```bash
# 载入离线镜像；归档内镜像标签固定为 sk5proxy:offline
docker load -i sk5proxy-linux-amd64.tar.gz

# 确认镜像在本地
docker image ls sk5proxy

# 用离线 compose 启动，不构建、不联网拉取
docker compose -f docker-compose.offline.yml up -d
```

`docker-compose.offline.yml` 关键点：

- `image: sk5proxy:offline`：必须与 `docker load` 得到的标签一致。
- `pull_policy: never`：从不联网拉取，镜像不在本地就直接报错（而不是去 registry 找）。
- 没有 `build:` 段：不会触发源码构建，因此也不会有 Go 模块下载。

### 1.3 验证

```bash
docker compose -f docker-compose.offline.yml ps
curl http://127.0.0.1:8081/healthz          # 期望返回 ok
curl http://127.0.0.1:8081/api/config       # 期望返回公共配置 JSON
```

`healthz` 由容器内 healthcheck 每 10s 探测一次（`wget -q -O - http://127.0.0.1:8081/healthz`）。`docker compose ps` 的 `STATUS` 列出现 `healthy` 只表示管理 HTTP 端点可响应。

> 该 healthcheck 只探测本服务自身的 `/healthz`，**不**对上游代理做任何健康检查或探活。

## 2. 从源码构建部署（联网环境）

目标机器能联网、或你想自己出镜像时：

```bash
docker compose up -d --build
```

默认 `docker-compose.yml` 带 `build:` 段，会用仓库根目录的 `Dockerfile` 构建 `sk5proxy:local`。构建期需要联网下载 Go 模块（`go mod download`）。若构建卡在模块下载，见 `docs/troubleshooting.md`。

## 3. 端口与发布

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

## 4. `.env` 配置

在 compose 文件同目录放 `.env`：

```dotenv
# 动态监听器范围对外发布到哪个宿主机地址
SK5_PROXY_BIND_IP=127.0.0.1
# 动态监听器端口范围（宿主发布范围要与 Web 里创建的端口对得上）
SK5_PROXY_PORT_RANGE=10080-10180
```

- 只想本机用：保持 `127.0.0.1`。
- 想给局域网用：改成宿主机的 LAN 网卡地址，见下一节。

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
  - 停止：`docker compose -f docker-compose.offline.yml down`
  - 重启：`docker compose -f docker-compose.offline.yml up -d`
  - 看日志：`docker compose -f docker-compose.offline.yml logs -f`
