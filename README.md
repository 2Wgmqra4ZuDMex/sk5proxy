# sk5proxy

轻量级多入口代理转换服务。每个 SOCKS5/HTTP 下游监听器都独立绑定地址、独立选择上游，并可在运行时通过 Web 面板创建、编辑、启停、切换或删除。无需数据库，也不依赖外部代理进程。

面向运维的完整文档见 `docs/`：

- `docs/deployment.md`：离线 Docker 部署与多监听器远程访问
- `docs/troubleshooting.md`：常见故障排查

## 快速开始（预构建镜像，推荐）

默认 `docker-compose.yml` **只使用镜像、不从源码构建**，镜像名为 `${SK5_IMAGE:-sk5proxy:offline}`，本地没有时允许从 registry 拉取。复制环境变量示例：

```bash
cp .env.example .env
```

完全离线时，从发布方取得与宿主架构一致的版本化归档和 `.sha256`，先校验再载入：

```bash
cd dist
sha256sum -c sk5proxy-v1.2.3-linux-amd64.tar.gz.sha256
docker load -i sk5proxy-v1.2.3-linux-amd64.tar.gz
cd ..

# 默认入口（推荐）
docker compose up -d
# 旧部署脚本仍可继续使用严格不拉取的兼容入口：
# docker compose -f docker-compose.offline.yml up -d

curl http://127.0.0.1:8081/healthz
curl http://127.0.0.1:8081/api/config
```

仓库当前保留的历史 amd64 归档已在本地整理为 `dist/sk5proxy-980be17-linux-amd64.tar.gz`（`dist/` 被 Git 忽略，根目录原件仍保留）。新归档用 `scripts/build-image.sh <版本>` 生成，文件名包含版本和架构，并带 SHA-256 校验文件。

需要显式从源码构建时，使用独立的开发 Compose；它保留相同服务名、卷、端口和运行参数：

```bash
docker compose -f docker-compose.build.yml up -d --build
```

Docker Hub 自动发布的配置、所需输入和完整离线流程见 `docs/deployment.md`。

## 端口

| 端口 | 用途 | 默认协议 | 说明 |
| --- | --- | --- | --- |
| `8081` | Web 管理面板 + `/api` + `/healthz` | TCP | 无入站认证，切勿发布到公网 |
| `1080` | 迁移用默认 SOCKS5 监听器 | TCP | 仅用于初始/旧配置迁移的默认端口 |
| `8080` | 迁移用默认 HTTP 监听器 | TCP | 仅用于初始/旧配置迁移的默认端口 |
| `10080-10180` | 动态监听器端口范围 | TCP | 可配置，Web 新建监听器须落在此范围 |

Compose 默认只把管理端口和默认端口发布到宿主机回环 `127.0.0.1`，动态范围通过 `.env` 控制发布地址：

```dotenv
SK5_PROXY_BIND_IP=127.0.0.1
SK5_PROXY_PORT_RANGE=10080-10180
```

Docker **不会**在容器运行后自动为 Web 新建的端口打洞。监听器端口必须落在启动容器时已发布的范围内；更改范围后要重建容器（`docker compose ... up -d`）。Dockerfile 里的 `EXPOSE` 只是镜像元数据，本身不发布任何端口。

## 配置与 API

持久配置格式（对应 `config.example.json`）：

```json
{
  "activeId": "",
  "upstreams": [
    {"id": "up-a", "name": "机房 A", "type": "http", "address": "proxy-a.example:3128"}
  ],
  "listeners": [
    {"id": "lan-http", "name": "LAN HTTP", "type": "http", "address": "0.0.0.0:10080", "upstreamId": "up-a", "enabled": true}
  ]
}
```

`activeId` 只是旧 API 的兼容字段：若非空，它必须指向一个已存在的 upstream，否则配置校验会失败。新连接的路由**只**读取各监听器自己的 `upstreamId`，与 `activeId` 无关。全新配置建议直接留空 `activeId`。

| 方法与路径 | 请求 | 成功 | 主要错误 |
| --- | --- | --- | --- |
| `GET /api/config` | 无 | `200` 公共配置（不含密码） | — |
| `PUT /api/config/upstreams` | 完整 upstream；编辑时省略 password 保留旧密码 | `200` 公共配置 | `400` 校验失败 |
| `DELETE /api/config/upstreams/{id}` | 无 | `200` 公共配置 | `404`；被监听器引用或是旧 active 时 `409` |
| `POST /api/config/activate` | `{"id":"up-a"}` | `200`，仅兼容旧 active | `404` |
| `PUT /api/config/listeners/{id}` | `{"name":"LAN HTTP","type":"http","address":"0.0.0.0:10080","upstreamId":"up-a","enabled":true}` | `200` 公共配置 | `400` 校验；绑定失败 `409`；保存失败 `500` |
| `POST /api/config/listeners/{id}/switch` | `{"upstreamId":"up-b"}` | `200` 公共配置 | `404` listener；无效 upstream `400` |
| `DELETE /api/config/listeners/{id}` | 无 | `200` 公共配置 | `404` |

`PUT listener` 同时承担创建、编辑、启用与禁用，请求是完整替换。ID 允许 `A-Z a-z 0-9 . _ -`，最长 64。类型仅 `socks5`/`http`，地址必须是 `host:port`。启用监听器会先试绑定，只有全部准备成功且配置原子落盘后才发布运行时变更；失败不改动磁盘，也不影响当前运行的路由。

### Web 里添加上游

在 Web 面板添加上游时，字段是独立的：

- **地址**：`host:port` 形式，例如 `proxy-a.example:3128`，不要把用户名密码写进地址。
- **用户名 / 密码**：单独两个字段，二者要么都填、要么都空（只填其一会被校验拒绝）。

多条上游各自独立，示例：

```json
"upstreams": [
  {"id": "up-a", "name": "机房 A", "type": "http",   "address": "proxy-a.example:3128"},
  {"id": "up-b", "name": "机房 B", "type": "socks5", "address": "10.0.0.9:1080", "username": "u", "password": "p"}
]
```

每个监听器通过自己的 `upstreamId`（或面板上的“切换上游”）独立选择走哪条上游，互不影响。切换只作用于该监听器后续的新连接。

旧版、没有 `listeners` 字段的持久配置在启动时会显式迁移为 `default-socks5` 与 `default-http`，地址取自 `SK5_SOCKS_ADDR`/`SK5_HTTP_ADDR`，两者继承旧 `activeId` 并立即写回磁盘。

## 连接语义与远程访问

- 容器内进程按环境变量把默认监听器绑定到 `0.0.0.0`（见 compose），但那只是**容器内**监听。客户端连接时要用**宿主机**的地址（回环或 LAN IP），能否连上取决于 Docker 端口发布，Docker 不会自动发布。
- 直接运行（非 Docker）时管理端默认 `127.0.0.1:8081`，默认监听器也在回环；要远程访问，必须把对应监听器地址显式设为 `0.0.0.0:端口` 或某块网卡 IP。
- 切换上游只影响之后的新拨号；已建立的 SOCKS5、HTTP CONNECT 隧道会沿用原上游直到连接结束。
- 禁用或删除监听器只停止接受新连接，不强制断开已有连接。
- 普通 HTTP 转发在路由或上游配置变化时会替换 transport 并关掉旧空闲连接，避免连接池跨路由复用。
- 上游为 HTTP 类型时，本服务只用 HTTP `CONNECT` 建立 TCP 隧道，仅承载 TCP。它不实现 UDP，也不是“把明文 HTTP 请求转发给上游 HTTP 代理”的通用 HTTP 代理链路。
- **无入站认证**：下游代理端口和管理 API 都不做认证。切勿把管理端（8081）发布到公网。若要把代理开放给其他机器，请用主机防火墙 / 云安全组只放行可信源 IP，并优先绑定专用 LAN/VPN IP。

### 对局域网开放（Docker）

想让同网段其他机器用代理，两步：

1. 在 `.env` 把动态范围发布到宿主机的 LAN 网卡地址：

   ```dotenv
   SK5_PROXY_BIND_IP=192.0.2.10
   SK5_PROXY_PORT_RANGE=10080-10180
   ```

   客户端连 `192.0.2.10:10080` 这类地址；容器内监听器仍写 `0.0.0.0:10080`（容器内 bind），对外能否连上由这里的发布地址决定。

2. 管理面板端口（8081）如需从别的机器打开，得在 compose 里把它那行的 `127.0.0.1` 改成对应 LAN IP。但由于面板无认证，**推荐用 SSH 隧道**而不是直接改映射：

   ```bash
   ssh -L 8081:127.0.0.1:8081 user@192.0.2.10
   # 然后本机浏览器访问 http://127.0.0.1:8081
   ```

务必配合防火墙 / 云安全组只放行需要的客户端 IP。注意 Docker 发布端口通常绕过 UFW 的默认规则（Docker 直接改 iptables 的 DOCKER 链），单靠 UFW 可能挡不住，详见 `docs/troubleshooting.md` 与 `docs/deployment.md` 的安全小节。

### 数据卷

配置持久化在名为 `sk5proxy-data` 的命名卷（挂到容器 `/data`）。卷名由 compose 项目名派生，想复用同一份配置，就在同一目录、用同一个 compose 项目名启动。`docker compose down` 保留卷，**不要用 `docker compose down -v`**，那会连同 `config.json` 一起删掉。

## 直接运行与开发

环境变量：`SK5_CONFIG_PATH`（默认 `config.json`）、`SK5_SOCKS_ADDR`（`127.0.0.1:1080`）、`SK5_HTTP_ADDR`（`127.0.0.1:8080`）、`SK5_WEB_ADDR`（`127.0.0.1:8081`）。前两个只用于首次创建和旧配置迁移，之后监听地址由持久配置管理。

```bash
docker run --rm -v "$PWD":/src -w /src golang:1.23 go test -race -shuffle=on -count=1 ./...
docker run --rm -v "$PWD":/src -w /src golang:1.23 go vet ./...
docker run --rm -v "$PWD":/src -w /src golang:1.23 go build -buildvcs=false ./cmd/sk5proxy
docker compose -f docker-compose.build.yml build
scripts/build-image.sh v1.2.3
```
