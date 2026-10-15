# Mac 上可被 AI 调用、用来启动 Docker 里 MySQL / Redis 的 Docker 管理工具

- 查阅日期:2026-10-10(产品信息、价格、版本可能变化,一律以当时官方页面为准)
- 范围:Docker Desktop(含 MCP Toolkit / MCP Gateway / Model Runner / Sandboxes)、OrbStack、Colima、Podman / Podman Desktop、Rancher Desktop、Docker 相关 MCP server、docker compose、Testcontainers
- 依据:仅官方文档、官方 GitHub 仓库 / API、许可与定价页、Docker Hub 官方镜像页。博客和二手文章不作为依据。
- 方法说明:官方网页由抓取工具做了摘要,其中价格、镜像标签、许可原文另用原始 HTML 或 GitHub / Docker Hub API 复核过(条目里写"已复核");没复核的写"未核实";查不到一手来源的写"无官方数据"。未核实清单集中在 2.3 节。
- 引用格式:每条结论后的 `[Sx](URL)` 是指向该来源的内联链接,完整列表见第 6 节。

---

## 1. 结论与推荐

**先说最重要的发现:要让 AI "启动" MySQL / Redis,不需要 MCP。** Claude Code 能直接在 Bash 里跑 `docker run` / `docker compose up -d --wait`;MCP 在这件事上反而更重、风险更集中。

1. **MCP 目录里没有"专门、结构化地起 MySQL / Redis 容器"的官方 server;唯一能起容器的官方条目是一个原样转发 Docker CLI 的通用 server。**
   - Docker MCP Catalog 里的 `mcp/redis`(Redis 官方出品)只**连接已存在**的 Redis 实例,53 个工具里没有一个会启动 / 部署实例 [S9](https://hub.docker.com/mcp/server/redis/overview)。
   - 目录里与 MySQL 相关的 `database-server` 也只通过 `DATABASE_URL` 连接已有数据库,来自个人仓库 `Souhar-dya/mcp-db-server`,不是 Docker 自己的项目 [S10](https://hub.docker.com/mcp/server/database-server/overview)。
   - 目录里作者标为 Docker Inc. 的 `docker` 条目只有一个工具 `docker`,参数是任意 `args` 数组(镜像 `docker:cli`)[S46](https://hub.docker.com/mcp/server/docker/overview);registry 仓库中它的定义还挂载了宿主 `/var/run/docker.sock` [S11](https://github.com/docker/mcp-registry/tree/main/servers)。即它能起 MySQL / Redis,但同时等于把整个 Docker 控制权交给模型(见第 5 节)。
   - 按名称过滤 registry 仓库 `servers/` 目录(共 328 项)中含 mysql / redis / docker / container / database 的只有:`SQLite`、`cloudflare-container`、`database-server`、`docker-docs`、`docker`、`dockerhub`、`redis-cloud`、`sqlite-mcp-server`、`redis`;我只读了其中 `docker`、`dockerhub`、`database-server`、`redis` 的定义,其余未逐项审阅 [S11](https://github.com/docker/mcp-registry/tree/main/servers)。
2. 社区的"容器管理类" MCP(`ckreiling/mcp-server-docker`、`QuantGeekDev/docker-mcp`)能起容器,但都不是 Docker 官方项目 [S12](https://github.com/ckreiling/mcp-server-docker)[S13](https://github.com/QuantGeekDev/docker-mcp)。

### 按场景推荐(每个场景 1 个主选 + 1 个备选)

| 场景 | 主选组合 | 备选 |
|---|---|---|
| **个人(非商业)Mac 本机开发,让 Claude Code 起库** | **Docker CLI + `compose.yaml` 入库,由 Claude Code 通过 Bash 调用**;运行时用 **OrbStack**(个人非商业免费 [S6](https://orbstack.dev/pricing))或 **Docker Desktop**(个人免费 [S3](https://docs.docker.com/subscription/desktop-license/)) | Docker Desktop + MCP Toolkit,仅用于让 AI 通过 `mcp/redis` 等 server **操作**已启动的库 [S1](https://docs.docker.com/ai/mcp-catalog-and-toolkit/toolkit/)[S9](https://hub.docker.com/mcp/server/redis/overview) |
| **公司 / 商业使用(含公司内部工具开发,如本仓库)** | **Colima(MIT)+ docker CLI + docker compose**,无商用许可门槛、无订阅费 [S14](https://github.com/abiosoft/colima)[S22](https://github.com/abiosoft/colima) | **Docker Desktop**:仅当公司"少于 250 人**且**年收入低于 1000 万美元"才免费,否则需订阅 [S3](https://docs.docker.com/subscription/desktop-license/);**OrbStack 商用一律付费**($8 / 用户 / 月,无规模门槛)[S6](https://orbstack.dev/pricing)[S6b](https://docs.orbstack.dev/licensing);或 Podman + Podman Desktop / Rancher Desktop(均 Apache-2.0)[S23](https://podman-desktop.io/docs/intro)[S24a](https://github.com/containers/podman)[S24b](https://github.com/containers/podman-desktop)[S25](https://github.com/rancher-sandbox/rancher-desktop) |
| **测试代码里起库(让 AI 写 / 跑集成测试)** | **Testcontainers**(Java 有 `MySQLContainer`,Redis 用 `GenericContainer`)[S19](https://java.testcontainers.org/modules/databases/mysql/)[S20](https://java.testcontainers.org/quickstart/junit_5_quickstart/),运行时用 Docker Desktop(官方自动探测、测试最充分)[S21](https://java.testcontainers.org/supported_docker_environment/) | OrbStack / Colima 等需按环境变量手工配置(文档只给出 Colima / Podman / Rancher Desktop 的配置,未提 OrbStack)[S21](https://java.testcontainers.org/supported_docker_environment/) |

**不推荐**:让 AI 通过"挂载 docker.sock 的 MCP server"来管理容器,见第 5 节。

---

## 2. 对比表

### 2.1 功能与性能对比

> "无官方数据" = 我没找到官方文档或可复现的一手数字。所有"性能"格只写官方明确陈述的内容。

| 工具 | AI 调用路径 | 官方 MCP | Mac 虚拟化 / 性能(一手来源) | 授权与收费 |
|---|---|---|---|---|
| **Docker Desktop** | CLI(`docker` / `docker compose`);MCP Gateway(`docker mcp`);默认**不创建** `/var/run/docker.sock`,需在 Settings > Advanced 勾选 [S4](https://docs.docker.com/desktop/setup/install/mac-install/)[S5](https://docs.docker.com/desktop/settings-and-maintenance/settings/) | 有:MCP Toolkit / MCP Gateway(MIT)[S1](https://docs.docker.com/ai/mcp-catalog-and-toolkit/toolkit/)[S2](https://github.com/docker/mcp-gateway),目录 300+ 个 server [S7](https://docs.docker.com/ai/mcp-catalog-and-toolkit/catalog/) | Docker VMM(Beta,官方推荐用于容器性能)/ Apple Virtualization;默认内存上限 = 宿主内存 50%,swap 1 GB;Resource Saver 空闲时关 VM,再启动 3-10 秒;VirtioFS 为默认文件共享,官方称文件系统操作时间"最多减少 98%"(未给测试条件)[S5b](https://docs.docker.com/desktop/settings-and-maintenance/settings/)。无启动耗时、常驻内存的官方数据 | Personal $0;Pro 年付 $9 / 月付 $11;Team 年付 $15 / 月付 $16;Business 年付 $24(每用户每月;已复核原始页面)[S3a](https://www.docker.com/pricing/)。免费使用条件:个人、教育、非商业开源、**少于 250 人且年收入低于 1000 万美元**的小企业 [S3](https://docs.docker.com/subscription/desktop-license/) |
| **OrbStack** | CLI(`docker` context `orbstack`;授权后可建 `/var/run/docker.sock` 软链);`orb` CLI [S16](https://docs.orbstack.dev/) | 无(未发现) | 轻量 Linux VM + 共享内核;官方称"2 秒启动"、空闲 CPU 约 0.1%、内存按需动态分配 [S17](https://docs.orbstack.dev/docker/)[S18](https://docs.orbstack.dev/features)。厂商自测基准:OrbStack v0.17.0 vs Docker Desktop v4.22.0,2023 年 8 月、M1 Max,**只有图表、无数字**,属厂商数据且版本很旧 [S19b](https://docs.orbstack.dev/benchmarks)。无独立第三方 / 最新数据 | 个人非商业免费;商业 Pro $8 / 用户 / 月($96 / 年,已复核);**商业、非营利、政府、自由职业者使用均需许可,无公司规模门槛**;一个用户最多 5 台设备 [S6](https://orbstack.dev/pricing)[S6b](https://docs.orbstack.dev/licensing) |
| **Colima** | CLI(`colima start` 后 `docker` 可直接用);socket 在 `~/.colima/default/docker.sock`(Testcontainers 文档给出)[S14](https://github.com/abiosoft/colima)[S21](https://java.testcontainers.org/supported_docker_environment/) | 无 | 基于 Lima,支持 `vz` + Rosetta;默认 VM 2 CPU / 2 GiB / 100 GiB。无性能数据:无官方数据 [S14](https://github.com/abiosoft/colima) | MIT,免费 [S14](https://github.com/abiosoft/colima)[S22](https://github.com/abiosoft/colima) |
| **Podman / Podman Desktop** | CLI(`podman`);Mac 上由 VM 内服务提供,官方称"监听 Docker API 客户端";`DOCKER_HOST` 指向 machine socket(Testcontainers 文档)[S26](https://podman.io/docs/installation)[S21](https://java.testcontainers.org/supported_docker_environment/) | 无(Podman Desktop 官方简介页未提 MCP;其 AI 功能是 AI Lab / 模型推理)[S23](https://podman-desktop.io/docs/intro) | Mac 需 VM;provider 有 `libkrun`(Mac 默认)与 `applehv` [S27](https://docs.podman.io/en/latest/markdown/podman-machine.1.html)。性能:无官方数据 | Podman 与 Podman Desktop 均 Apache-2.0 [S24d](https://github.com/testcontainers/testcontainers-java) |
| **Rancher Desktop** | CLI(选 dockerd 则用 `docker`;选 containerd 则用 `nerdctl`)[S15](https://docs.rancherdesktop.io/);`rdctl` | 无(官方简介页未提 MCP)[S15](https://docs.rancherdesktop.io/) | Mac 上 VM 类型可选 **VZ(默认)** 或 QEMU;VZ 下可开 Rosetta [S28](https://docs.rancherdesktop.io/ui/preferences/virtual-machine/emulation/)。性能:无官方数据 | Apache-2.0 [S25](https://github.com/rancher-sandbox/rancher-desktop) |
| **docker compose** | 纯 CLI,任何能跑 Bash 的 AI 都可调用;`up --wait` 等待 running / healthy [S29](https://docs.docker.com/compose/)[S30](https://docs.docker.com/reference/cli/docker/compose/up/) | 不适用 | 取决于所选运行时 | Compose 仓库 Apache-2.0 [S24c](https://github.com/docker/compose) |
| **Testcontainers** | 库(Java / Go 等),由测试代码在运行时拉起容器;需要一个 Docker API 兼容运行时 [S19](https://java.testcontainers.org/modules/databases/mysql/)[S21](https://java.testcontainers.org/supported_docker_environment/) | 不适用 | 取决于所选运行时 | testcontainers-java:MIT [S24d](https://github.com/testcontainers/testcontainers-java) |
| **Docker 相关 MCP server** | MCP stdio;见 3.8 | 已核实的 Docker 官方 server:Docker Hub MCP(Apache-2.0,仅查 Hub,不管容器)[S31](https://github.com/docker/hub-mcp)、目录里作者为 Docker Inc. 的 `docker`(CLI 透传)[S46](https://hub.docker.com/mcp/server/docker/overview);Redis 官方 MCP(MIT,仅连接)[S9](https://hub.docker.com/mcp/server/redis/overview) | 不适用 | 各仓库各自协议,见 3.8 |

### 2.2 授权与收费汇总(2026-10-10 查阅,以官方页为准)

| 工具 | 个人 | 商用 / 公司 | 来源 |
|---|---|---|---|
| Docker Desktop | 免费 | 小企业(<250 人**且**<$10M 年收入)免费;更大组织的专业使用、政府机构需付费(Pro / Team / Business) | [S3](https://docs.docker.com/subscription/desktop-license/)[S3a](https://www.docker.com/pricing/) |
| OrbStack | 免费(非商业) | 必须购买,Pro $8 / 用户 / 月,无规模门槛;Enterprise 需联系销售 | [S6](https://orbstack.dev/pricing)[S6b](https://docs.orbstack.dev/licensing) |
| Colima | MIT | MIT | [S22](https://github.com/abiosoft/colima) |
| Podman / Podman Desktop | Apache-2.0 | Apache-2.0 | [S24a](https://github.com/containers/podman)[S24b](https://github.com/containers/podman-desktop) |
| Rancher Desktop | Apache-2.0 | Apache-2.0 | [S25](https://github.com/rancher-sandbox/rancher-desktop) |
| Docker Compose / Testcontainers | Apache-2.0 / MIT | 同左 | [S24c](https://github.com/docker/compose)[S24d](https://github.com/testcontainers/testcontainers-java) |
| Docker MCP Gateway | MIT | MIT | [S2](https://github.com/docker/mcp-gateway) |
| Docker Sandboxes | CLI 与本地算力免费(含商用) | 同左;云端按量付费 | [S34](https://docs.docker.com/ai/sandboxes/) |

补充:开源运行时(Colima、Podman、Rancher Desktop)的许可只覆盖这些工具本身;你拉取的镜像(Redis 8 的三选一授权等)有各自的许可,见第 4.2 节。

### 2.3 未核实 / 无官方数据清单

- 所有运行时的启动耗时、空闲内存、数据库吞吐:无官方数据(OrbStack 仅有厂商自测的图表和宣称,Docker Desktop 仅有设置默认值与 Resource Saver 重启 3-10 秒)。
- Colima / Podman / Rancher Desktop 的性能与资源占用:无官方数据。
- `database-server` 的授权协议。
- `mcp-server-docker` 是否识别 Docker context;Testcontainers 自动探测与 Docker Desktop 默认不建 `/var/run/docker.sock` 的实际交互。
- Docker Sandboxes 内能否再用 Docker 起 MySQL / Redis。
- MySQL 镜像里 MySQL 软件的具体协议(Hub 页只指向 mysql.com/about/legal);MySQL / Redis healthcheck 命令与 Redis `requirepass` 的官方镜像页依据。
- `mcp/redis` 在容器内连接宿主发布端口的具体填法。
- Rancher Desktop `~/.rd/docker.sock` 的 Rancher 官方文档出处(仅 Testcontainers 文档提到)。
- 其余产品的 MCP 能力(OrbStack、Colima、Podman、Rancher Desktop):官方文档页面检索未见,不排除新版本新增;以各自当前文档为准。

---

## 3. 各工具详情

### 3.1 Docker Desktop

**AI 可调用性**
- CLI:任何能执行 Bash 的 AI(Claude Code 等)可直接调用 `docker` / `docker compose`。
- 注意 socket:新版 Docker Desktop 安装后默认**不**创建 `/var/run/docker.sock`,可在 Settings > Advanced 勾选"Allow the default Docker socket to be used"(需密码)[S4](https://docs.docker.com/desktop/setup/install/mac-install/)[S5](https://docs.docker.com/desktop/settings-and-maintenance/settings/)。依赖该路径的第三方工具(例如某些 MCP server)可能连不上,需要开启或设置 `DOCKER_HOST`。
- MCP Toolkit:要求 Docker Desktop 4.62 及以上(官方文档)[S1](https://docs.docker.com/ai/mcp-catalog-and-toolkit/toolkit/);`mcp-gateway` README 写 4.59+ [S2](https://github.com/docker/mcp-gateway);目录里 Redis server 页写"自动添加需要 4.43+" [S9](https://hub.docker.com/mcp/server/redis/overview)。三处版本号各自属于不同功能 / 页面,未做统一,以实际界面为准。
- MCP Gateway(`docker mcp` CLI 插件,MIT)是"AI 客户端 → Gateway → 在 Docker 容器里运行的 MCP server"的单一入口 [S2](https://github.com/docker/mcp-gateway)。官方 README 给出的接入 Claude Code 的命令:
  ```bash
  docker mcp catalog pull mcp/docker-mcp-catalog
  docker mcp profile create --name dev-tools --server catalog://mcp/docker-mcp-catalog/github
  docker mcp client connect claude-code --profile dev-tools --global
  ```
  [S2](https://github.com/docker/mcp-gateway) 另可用 `docker mcp gateway run --profile <name>` 以 stdio 方式运行 [S1](https://docs.docker.com/ai/mcp-catalog-and-toolkit/toolkit/)[S2](https://github.com/docker/mcp-gateway)。
- 每个 profile 可做工具白名单 / 黑名单,例如 `docker mcp profile tools <id> --enable github.create_issue` [S2](https://github.com/docker/mcp-gateway)。
- Dynamic MCP:连上 Gateway 后,AI 可在对话中搜索目录并添加 server [S1](https://docs.docker.com/ai/mcp-catalog-and-toolkit/toolkit/)。
- Claude Code 自身添加 stdio MCP 的语法是 `claude mcp add --transport stdio <name> -- <command> [args...]`;项目级写入 `.mcp.json`,需用户批准 [S32](https://code.claude.com/docs/en/mcp)。

**与"起数据库"的关系**:MCP Toolkit 解决的是"AI 用工具",不是"AI 起容器"。要起 MySQL / Redis,仍用 CLI / compose。

**安全特性(官方)**:`mcp/` 命名空间下的镜像由 Docker 构建、签名并附 SBOM;每个工具容器上限 1 CPU / 2 GB 内存;server 默认无宿主文件系统访问,由用户显式授权挂载;含密钥的请求 / 响应会被拦截 [S1](https://docs.docker.com/ai/mcp-catalog-and-toolkit/toolkit/)。

**相关但与起库无关的两项**
- **Docker Model Runner**:管理 / 运行 / 部署 AI 模型(llama.cpp 等),Mac 要求 Apple Silicon,提供 OpenAI / Ollama 兼容 API。官方警告该 API **无认证**,同网络里的任何客户端(含其他容器)都可拉取、加载模型并推理 [S33](https://docs.docker.com/ai/model-runner/)。与启动 MySQL / Redis 无关。
- **Docker Sandboxes(`sbx` CLI)**:在隔离环境里运行 AI 编码代理(Claude Code 在支持列表中);本地要求 Apple silicon + macOS 14+,`brew install docker/tap/sbx`、`sbx login`;CLI 与本地算力免费(含商用),云端按量付费,集中的组织策略为另一项付费订阅 [S34](https://docs.docker.com/ai/sandboxes/)[S35](https://docs.docker.com/ai/sandboxes/install/)[S36](https://docs.docker.com/ai/sandboxes/agents/)。沙箱内是否能再用 Docker 起 MySQL / Redis:未核实。

**性能(官方)**:见对比表。Docker VMM 目前标注 Beta [S5b](https://docs.docker.com/desktop/settings-and-maintenance/settings/)。启动耗时、空闲内存:无官方数据。

**授权与收费**:见对比表与 [S3](https://docs.docker.com/subscription/desktop-license/)[S3a](https://www.docker.com/pricing/)。免费条件要求"少于 250 人"**且**"年收入低于 1000 万美元",两项都要满足;政府机构、更大组织内的专业使用需付费订阅 [S3](https://docs.docker.com/subscription/desktop-license/)。定价页本身对 Personal 只写"For individual developers",没有写明员工 / 营收阈值,阈值在许可文档 [S3](https://docs.docker.com/subscription/desktop-license/)[S3a](https://www.docker.com/pricing/)。

### 3.2 OrbStack

- 定位:Docker Desktop / WSL 的替代,可运行容器、Linux 机器和 Kubernetes [S16](https://docs.orbstack.dev/)。
- AI 可调用性:自动创建名为 `orbstack` 的 Docker context,终端里的 `docker` 命令自动使用;授权管理员后会建立 `/var/run/docker.sock` 软链,方便第三方工具 [S17](https://docs.orbstack.dev/docker/)。自带 `docker compose` 与 buildx(仅在你没有可用版本时安装)[S17](https://docs.orbstack.dev/docker/)。官方提供 `orb` CLI(文档入口为"Command line & CI")。**未发现官方 MCP**,功能页也未提 MCP / AI 代理 [S18](https://docs.orbstack.dev/features)。
- 官方也警告:若把引擎通过 TCP 暴露,"extremely dangerous",应改用 SSH 或带客户端认证的 TLS [S17](https://docs.orbstack.dev/docker/)。
- 性能:官方陈述为"2 秒启动"、空闲约 0.1% CPU、内存"完全动态分配"(不用的内存还给 macOS),新安装占磁盘 <10 MB;效率页**没有给出测试方法** [S18](https://docs.orbstack.dev/features)[S18b](https://docs.orbstack.dev/efficiency)。厂商基准见对比表,属厂商自测、版本为 2023 年的旧版本 [S19b](https://docs.orbstack.dev/benchmarks)。
- 授权:个人非商业免费;商业使用($8 / 用户 / 月)。条款原文要求以自由职业者身份专业使用、为商业 / 非营利 / 政府实体使用,或相关工作年收入超过 $10,000,都需要许可;无公司规模阈值;新安装有 30 天 Pro 试用 [S6](https://orbstack.dev/pricing)[S6b](https://docs.orbstack.dev/licensing)。

### 3.3 Colima

- Containers on Lima:支持 Docker(可选 Kubernetes)、containerd、Incus;还有面向 AI 工作负载的 GPU 加速容器(`krunkit`,Apple Silicon、macOS 13+)[S14](https://github.com/abiosoft/colima)。
- 安装与启动:`brew install colima`,`colima start`;默认 2 CPU / 2 GiB / 100 GiB,可用 `--cpu --memory --disk` 调整;`--vz-rosetta` 用于 Rosetta [S14](https://github.com/abiosoft/colima)。装好 `docker` 客户端(`brew install docker`)后无需额外设置即可使用 [S14](https://github.com/abiosoft/colima)。
- socket 路径:README 不提;Testcontainers 文档使用 `unix://${HOME}/.colima/default/docker.sock` [S21](https://java.testcontainers.org/supported_docker_environment/)。
- 官方 MCP:无。性能:无官方数据。授权:MIT [S14](https://github.com/abiosoft/colima)[S22](https://github.com/abiosoft/colima);其底层 Lima 为 Apache-2.0 [S24e](https://github.com/lima-vm/lima)。最新版 v0.10.3(2026-06-04)[S22](https://github.com/abiosoft/colima)。

### 3.4 Podman / Podman Desktop

- Mac 上每个 Podman machine 由一个 VM 承载,`podman machine init` 然后 `podman machine start`;官方称 Mac 与 Windows 上的 Podman "监听 Docker API 客户端",可被基于 Docker 的工具使用,但该页没给 socket 路径 [S26](https://podman.io/docs/installation)。
- Provider:`libkrun`(Mac 默认)、`applehv` [S27](https://docs.podman.io/en/latest/markdown/podman-machine.1.html)。
- Testcontainers 文档:Mac 上 `DOCKER_HOST=unix://$(podman machine inspect --format '{{.ConnectionInfo.PodmanSocket.Path}}')`,并设 `TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock`;rootless 时还需 `TESTCONTAINERS_RYUK_DISABLED=true` [S21](https://java.testcontainers.org/supported_docker_environment/)。
- Podman Desktop 文档有"Migrating from Docker"章节(含 `DOCKER_HOST`、Docker 兼容性、Compose)[S23](https://podman-desktop.io/docs/intro)。官方 MCP:简介页未提 [S23](https://podman-desktop.io/docs/intro);其 AI Lab 用于本地模型推理。
- 性能:无官方数据。授权:Apache-2.0(podman 与 podman-desktop 两个仓库)[S24a](https://github.com/containers/podman)[S24b](https://github.com/containers/podman-desktop)。官方安装页**不推荐** Homebrew 安装 Podman(理由:社区维护),推荐 pkg 安装包 [S26](https://podman.io/docs/installation)。

### 3.5 Rancher Desktop

- 可选容器引擎:dockerd(moby)提供 Docker API 与 `docker` CLI;containerd 提供 `nerdctl`。同时只能用其一,切换时镜像 / 工作负载不迁移 [S15](https://docs.rancherdesktop.io/)[S15b](https://docs.rancherdesktop.io/ui/preferences/container-engine/general/)。内置 k3s [S15](https://docs.rancherdesktop.io/)。
- Mac 虚拟化:VM 类型 VZ(默认,Apple Virtualization.framework)或 QEMU;VZ 下 Apple Silicon 可启用 Rosetta [S28](https://docs.rancherdesktop.io/ui/preferences/virtual-machine/emulation/)。
- socket:Testcontainers 文档中非管理员 + VZ 的配置使用 `~/.rd/docker.sock` [S21](https://java.testcontainers.org/supported_docker_environment/)。(Rancher 自家文档该页未覆盖,未核实。)
- 官方 MCP:简介页未提。性能:无官方数据。授权:Apache-2.0;最新版 v1.24.0(2026-07-29)[S25](https://github.com/rancher-sandbox/rancher-desktop)。

### 3.6 docker compose

- Compose 在一个 YAML 文件里定义服务、网络、卷,一条命令创建并启动 [S29](https://docs.docker.com/compose/)。
- `docker compose up --wait`:"Wait for services to be running|healthy. Implies detached mode." 另有 `--wait-timeout`、`--detach` [S30](https://docs.docker.com/reference/cli/docker/compose/up/)。对 AI 来说关键是:**命令幂等、可读、可入库、可审查**,比让 AI 临场拼 `docker run` 参数更安全。
- healthcheck 语法见 Compose 规范(`test` / `interval` / `timeout` / `retries` / `start_period`)[S37](https://docs.docker.com/reference/compose-file/services/#healthcheck)。

### 3.7 Testcontainers

- Java:`new MySQLContainer("mysql:8.0.36")`,依赖 `org.testcontainers:testcontainers-mysql:2.0.5`(该 JAR 不带 MySQL 驱动,需自备)。默认 root 密码为 `test` [S19](https://java.testcontainers.org/modules/databases/mysql/)。
- Redis:官方快速开始用 `GenericContainer` + `DockerImageName.parse(...)` + `.withExposedPorts(6379)`,取端口用 `getHost()` / `getFirstMappedPort()`,文档建议不要硬编码 `localhost` [S20](https://java.testcontainers.org/quickstart/junit_5_quickstart/)。(MySQL 模块页未提 Redis;Redis 没有 Java 官方模块页可引,故用通用容器。)
- 运行时:Docker(Linux)与 Docker Desktop(Mac / Windows)被主动测试并自动探测;其他运行时不属主要测试流程,"部分功能可能不可用"且可能需手工配置 [S21](https://java.testcontainers.org/supported_docker_environment/)。
- 与 socket 的交互:Testcontainers 文档称 Docker Desktop 无需额外配置即可自动探测 [S21](https://java.testcontainers.org/supported_docker_environment/),但新版 Docker Desktop 默认不创建 `/var/run/docker.sock` [S4](https://docs.docker.com/desktop/setup/install/mac-install/);两者叠加时的实际表现:未核实。若连不上,退路是在 Docker Desktop 设置里开启默认 socket [S5](https://docs.docker.com/desktop/settings-and-maintenance/settings/),或设置 `DOCKER_HOST`(Testcontainers 的探测顺序首先读 `DOCKER_HOST` [S21](https://java.testcontainers.org/supported_docker_environment/))。
- Docker 的 Pro 及以上订阅含 Testcontainers Cloud 分钟数(Pro 100 分钟、Team 500、Business 1,500)[S3a](https://www.docker.com/pricing/)——与本地起库无关,仅供参考。
- 授权:testcontainers-java MIT [S24d](https://github.com/testcontainers/testcontainers-java)。

### 3.8 Docker 相关 MCP server

| Server | 来源 / 维护方 | 能否启动容器 | 授权 | 备注 |
|---|---|---|---|---|
| Docker Hub MCP(`docker/hub-mcp`,目录里 `mcp/dockerhub`) | Docker 官方仓库 | **否**:只做 Hub 搜索、命名空间、仓库、tag、Hardened Images 查询,没有 start / stop / build 工具 | Apache-2.0 | stdio 默认;HTTP 默认只绑回环并要 `MCP_AUTH_TOKEN` [S31](https://github.com/docker/hub-mcp)[S11](https://github.com/docker/mcp-registry/tree/main/servers) |
| `docker`(Docker MCP Catalog 条目;定义在 `docker/mcp-registry` 的 `servers/docker`) | 目录页作者:Docker Inc.;唯一工具 `docker`,参数 `args` 数组,镜像 `docker:cli` [S46](https://hub.docker.com/mcp/server/docker/overview) | 能(任意 `docker` 子命令) | 目录页写 Apache-2.0 [S46](registry 仓库本身为 MIT [S8](https://github.com/docker/mcp-registry)) | registry 中的定义挂载 `/var/run/docker.sock`(目录页未写,以仓库为准)[S11](https://github.com/docker/mcp-registry/tree/main/servers);Docker Desktop 4.43+ 可自动添加 [S46](https://hub.docker.com/mcp/server/docker/overview) |
| `mcp/redis` | Redis 官方(`redis/mcp-redis`),Docker 打包 | **否**:只连接已有 Redis(`REDIS_HOST` 等环境变量),53 个数据操作工具 | MIT | 密码走 secret `REDIS_PWD` [S9](https://hub.docker.com/mcp/server/redis/overview) |
| `database-server` | 个人仓库 `Souhar-dya/mcp-db-server` | **否**:`DATABASE_URL` 连接 PostgreSQL / MySQL / SQLite | 未核实 | 目录里唯一与 MySQL 相关的条目,非 Docker 自家 [S10](https://hub.docker.com/mcp/server/database-server/overview) |
| `ckreiling/mcp-server-docker` | 个人仓库,社区项目 | **能**:容器 / 镜像 / 网络 / 卷的增删改查,run / recreate / start / stop,另有 `docker_compose` prompt | GPL-3.0 | 最近推送 2026-08-07,最新发布 v0.3.0;故意不支持 `--privileged`、`--cap-add/drop`;README 自己写 "Docker is not a secure sandbox" [S12](https://github.com/ckreiling/mcp-server-docker)[S12b](https://github.com/ckreiling/mcp-server-docker) |
| `QuantGeekDev/docker-mcp` | 个人仓库,社区项目 | 能:`create-container`、`deploy-compose`、`get-logs`、`list-containers` | MIT | **最近推送 2024-12-14,疑似不再维护**;README 自述缺少卷 / 网络管理、健康检查、重启策略、资源限制 [S13](https://github.com/QuantGeekDev/docker-mcp)[S13b](https://github.com/QuantGeekDev/docker-mcp) |

> 实用提示:`mcp-server-docker` 的代码使用 `docker.from_env()` 创建客户端,README 的远程示例设置 `DOCKER_HOST` [S12b](https://github.com/ckreiling/mcp-server-docker)。它是否识别 Docker CLI 的 context(例如 Docker Desktop 默认不建 `/var/run/docker.sock` 时):未核实,若连不上请先设 `DOCKER_HOST` 或开启默认 socket。

---

## 4. 启动 MySQL / Redis 的示例

镜像来自 Docker Hub 官方 library 页:`mysql` [S38](https://hub.docker.com/_/mysql)、`redis` [S39](https://hub.docker.com/_/redis)。两者都含 arm64(`arm64v8`),在 Apple Silicon 上无需 x86 模拟 [S38](https://hub.docker.com/_/mysql)[S39](https://hub.docker.com/_/redis);Hub API 复核:`mysql:8.4`、`redis:8.10` 标签存在,均含 `arm64`(2026-10-10 查询)[S40a](https://hub.docker.com/v2/namespaces/library/repositories/mysql/tags)[S40b](https://hub.docker.com/v2/namespaces/library/repositories/redis/tags)。

注意:Hub 页当前 MySQL 标签包含 `26.7`(innovation)、`9.7`(`lts`)、`8.4`;为避免随 `latest` 漂移,以下固定 `mysql:8.4` 与 `redis:8.10`,请按需要的主版本自行调整 [S38](https://hub.docker.com/_/mysql)[S39](https://hub.docker.com/_/redis)。

### 4.1 最小 `docker run`

```bash
# MySQL(官方页示例命令的形态,补上端口和回环绑定)
docker run --name some-mysql -e MYSQL_ROOT_PASSWORD=my-secret-pw \
  -p 127.0.0.1:3306:3306 -d mysql:8.4

# Redis(官方页示例命令的形态,补上端口和回环绑定)
docker run --name some-redis -p 127.0.0.1:6379:6379 -d redis:8.10
```
- `MYSQL_ROOT_PASSWORD` 为必填;`MYSQL_DATABASE`、`MYSQL_USER` + `MYSQL_PASSWORD` 可选(二者需同时给才会建用户);变量支持 `_FILE` 后缀从文件读取 [S38](https://hub.docker.com/_/mysql)。`MYSQL_ALLOW_EMPTY_PASSWORD` 官方页不推荐 [S38](https://hub.docker.com/_/mysql)。
- 数据目录:MySQL `/var/lib/mysql`;Redis `/data`(`VOLUME`)[S38](https://hub.docker.com/_/mysql)[S39](https://hub.docker.com/_/redis)。
- 绑定 `127.0.0.1`:Docker 官方说明不写主机 IP 时默认发布到所有地址,"potentially making them available to the outside world",仅回环绑定则"only the Docker host can access" [S41](https://docs.docker.com/engine/network/port-publishing/)。Redis 官方页提醒:protected mode 默认关闭,暴露端口且无密码时任何人可访问 [S39](https://hub.docker.com/_/redis)。
- MySQL 容器初始化完成前不接受连接,依赖它的程序需要重试 [S38](https://hub.docker.com/_/mysql)。

### 4.2 推荐:`compose.yaml`(入库,让 AI 只执行 `docker compose up -d --wait`)

```yaml
services:
  mysql:
    image: mysql:8.4
    environment:
      MYSQL_ROOT_PASSWORD: example     # 官方页 compose 示例的值,仅限本机开发
      MYSQL_DATABASE: app
    ports:
      - "127.0.0.1:3306:3306"
    volumes:
      - mysql-data:/var/lib/mysql
    healthcheck:                        # 见下方说明:命令为作者设计,非官方页内容
      test: ["CMD", "mysqladmin", "ping", "-h", "127.0.0.1", "--silent"]
      interval: 5s
      timeout: 3s
      retries: 20
      start_period: 20s

  redis:
    image: redis:8.10
    command: ["redis-server", "--save", "60", "1", "--loglevel", "warning"]
    ports:
      - "127.0.0.1:6379:6379"
    volumes:
      - redis-data:/data
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      timeout: 3s
      retries: 10

volumes:
  mysql-data:
  redis-data:
```

```bash
docker compose up -d --wait      # 等待 running / healthy [S30](https://docs.docker.com/reference/cli/docker/compose/up/)
docker compose ps
docker compose down              # 默认不删命名卷;-v/--volumes 会删除 Compose 文件 volumes 段声明的命名卷及匿名卷 [S47](https://docs.docker.com/reference/cli/docker/compose/down/)
```

出处与未核实项:
- MySQL 的 compose 形态、环境变量:[S38](https://hub.docker.com/_/mysql);Redis 的 `redis-server --save 60 1 --loglevel warning` 与 `/data` 挂载:[S39](https://hub.docker.com/_/redis)。
- `healthcheck` 的语法来自 Compose 规范 [S37](https://docs.docker.com/reference/compose-file/services/#healthcheck)。**`mysqladmin ping` 与 `redis-cli ping` 作为健康探针的具体命令,不在 Hub 官方页上**(Hub 的 MySQL 页没有记录 healthcheck [S38](https://hub.docker.com/_/mysql));Redis 的 `redis-cli` 在 Hub 页示例里出现过 [S39](https://hub.docker.com/_/redis)。MySQL 探针请自行验证,尤其是首次初始化期间是否过早报 healthy:未核实。
- Redis 密码(`requirepass`)不在 Hub 官方页内,示例未设置;如需密码,请按 Redis 官方文档在配置文件里设置,Hub 页建议暴露到公网时必须设置密码 [S39](https://hub.docker.com/_/redis)。
- Redis 8 起为 RSALv2 / SSPLv1 / AGPLv3 三选一授权(7.4.x-7.8.x 为 RSALv2 或 SSPLv1,≤7.2.4 为 BSD-3-Clause;已用 Hub API 返回的页面原文复核,措辞即如此)[S39](https://hub.docker.com/_/redis)[S40b](https://hub.docker.com/v2/namespaces/library/repositories/redis/tags)。商用前请自行评估。
- MySQL 镜像:Hub 页 License 一节只指向 mysql.com/about/legal,并说明镜像里还含其他许可的软件、使用者需自行确保合规,本身不写具体协议名 [S40a](https://hub.docker.com/v2/namespaces/library/repositories/mysql/tags)。具体协议:未核实。

### 4.3 让 Claude Code 能运行这些命令(推荐的最小配置)

Claude Code 官方文档:`docker` 与沙箱不兼容,应用 `excludedCommands` 把需要的 docker 命令移出沙箱,且匹配模式越窄越好 [S42](https://code.claude.com/docs/en/sandboxing):

```json
{
  "sandbox": {
    "excludedCommands": ["docker compose up *", "docker compose down *", "docker compose ps *"]
  }
}
```
- 官方示例写的是 `"docker compose *"`;上面是我按"越窄越好"原则收窄后的写法。匹配规则见 [S42](https://code.claude.com/docs/en/sandboxing):模式用与 `Bash(...)` 权限规则相同的语法;不带通配符的模式是精确匹配;以 ` *` 结尾的模式匹配"带或不带参数"的命令(官方原话:`docker *` matches `docker` with or without arguments),因此 `docker compose up *` 也匹配不带参数的 `docker compose up`;整条命令里每一段都要匹配,否则仍在沙箱内。
- 沙箱默认不允许命令直连宿主上、沙箱外运行的服务(例如容器里的数据库);macOS 上要设 `network.allowLocalBinding: true`,但这会让沙箱命令可连接 localhost 上的任意端口,且官方提示无需认证的本地服务可能被利用 [S42](https://code.claude.com/docs/en/sandboxing)。

### 4.4 通过 MCP Gateway 让 AI **操作**已启动的 Redis(而非启动)

```bash
docker mcp profile create --name data-tools --server catalog://mcp/docker-mcp-catalog/redis
docker mcp client connect claude-code --profile data-tools --global
```
- 命令形态来自官方 README 的 GitHub 例子 [S2](https://github.com/docker/mcp-gateway);把 `github` 换成 `redis` 是**我的推导**,条目名以 `docker mcp catalog` 实际显示为准(`servers/redis` 存在于 registry [S11](https://github.com/docker/mcp-registry/tree/main/servers))。
- 该 server 通过 `REDIS_HOST`(默认 `127.0.0.1`)等变量连接;但它运行在容器里,容器内的 `127.0.0.1` 指向容器自己。需要访问宿主发布的端口时,Docker 官方说明用 `host.docker.internal` [S43](https://docs.docker.com/desktop/features/networking/networking-how-tos/)。具体该怎么填:未核实。

### 4.5 Testcontainers(Java)

```java
// MySQL:官方 MySQL 模块
// Maven: org.testcontainers:testcontainers-mysql:2.0.5(scope test),另需自备 MySQL 驱动 [S19](https://java.testcontainers.org/modules/databases/mysql/)
var mysql = new MySQLContainer("mysql:8.4");
mysql.start();
String url = mysql.getJdbcUrl();

// Redis:官方快速开始用 GenericContainer,取地址用 getHost()/getFirstMappedPort() [S20](https://java.testcontainers.org/quickstart/junit_5_quickstart/)
var redis = new GenericContainer<>(DockerImageName.parse("redis:8.10")).withExposedPorts(6379);
redis.start();
String host = redis.getHost();
int port = redis.getFirstMappedPort();
```
- `MySQLContainer("mysql:8.0.36")` 是官方页的写法,这里换成 `mysql:8.4` 属替换镜像标签,未单独验证与模块 2.0.5 的兼容。
- 使用 OrbStack / Colima / Podman / Rancher Desktop 时,Testcontainers 文档给出的是 `DOCKER_HOST`、`TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE`、`TESTCONTAINERS_HOST_OVERRIDE` 等环境变量组合(Colima / Podman / Rancher Desktop 有示例,OrbStack 未提及)[S21](https://java.testcontainers.org/supported_docker_environment/)。

---

## 5. 安全注意:让 AI 控制 Docker 的风险

1. **Docker 守护进程权限等同于宿主的高权限。** Docker 官方文档:守护进程默认需要 root,"只有可信用户才应被允许控制你的 Docker 守护进程";容器可以把宿主 `/` 整个挂载进去并"不受限制地修改宿主文件系统" [S44](https://docs.docker.com/engine/security/#docker-daemon-attack-surface)。结论:能调 Docker 的 AI = 能读写你整个磁盘的 AI,不论它是通过 CLI、MCP 还是 API。
2. **挂载 `docker.sock` 等于交出控制权。** Claude Code 官方安全说明:允许沙箱访问 `/var/run/docker.sock` "effectively grants access to the host system" [S42](https://code.claude.com/docs/en/sandboxing)。Docker 目录里的 `docker` 条目就是挂载 socket + 任意参数 [S11](https://github.com/docker/mcp-registry/tree/main/servers);`mcp-server-docker` 的 Docker 运行方式同样挂载 socket,README 建议不要给 LLM 创建的容器放敏感数据(API key、数据库密码),并称 "Docker is not a secure sandbox" [S12b](https://github.com/ckreiling/mcp-server-docker)。
3. **MCP Toolkit 的默认隔离不覆盖"挂载 socket 的 server"。** MCP Toolkit 给每个工具容器 1 CPU / 2 GB 上限、默认无宿主文件访问 [S1](https://docs.docker.com/ai/mcp-catalog-and-toolkit/toolkit/);这些保证对挂载了 socket 的 `docker` 条目无意义,因为它能通过 Docker 再起任意容器。
4. **Claude Code 的沙箱与 MCP。** 官方说明:沙箱只覆盖 shell 命令,文件工具、MCP server、hook 在沙箱**之外**运行 [S42](https://code.claude.com/docs/en/sandboxing);`docker` 命令又需用 `excludedCommands` 移出沙箱,被排除的命令以你的完整权限运行,宽泛的 `docker *` 覆盖该工具能做的一切;若模式覆盖了会读取工作目录文件的工具(如 `docker compose` 读 compose 文件),Claude 可以先写文件再在沙箱外执行它 [S42](https://code.claude.com/docs/en/sandboxing)。缓解:模式尽量窄、关键命令加 ask 规则逐次确认、compose 文件入库并审查。
5. **项目级 MCP 配置要审批。** `.mcp.json` 里的 server 在交互会话中需用户批准;但**非交互运行(`claude -p`、Agent SDK、云会话)不弹批准**,项目 server 会直接加载 [S32](https://code.claude.com/docs/en/mcp)。把能起容器的 MCP 写进 `.mcp.json` 前要想清楚这一点。官方原话建议:连接前确认你信任每个 server [S32](https://code.claude.com/docs/en/mcp)。
6. **不要把 Docker API 暴露到 TCP。** Docker 文档:HTTP 暴露必须用 HTTPS 与证书,否则守护进程启动即失败;SSH 是替代方案 [S44](https://docs.docker.com/engine/security/#docker-daemon-attack-surface)。OrbStack 文档同样称 TCP 暴露"extremely dangerous" [S17](https://docs.orbstack.dev/docker/)。
7. **端口绑定。** 默认发布到所有网卡,可能被外部访问 [S41](https://docs.docker.com/engine/network/port-publishing/)。本地开发库一律写 `127.0.0.1:端口:端口`;Redis 默认无密码 + protected mode 关闭 [S39](https://hub.docker.com/_/redis)。
8. **Enhanced Container Isolation** 需要登录并订阅 Docker Business,默认关闭 [S5b](https://docs.docker.com/desktop/settings-and-maintenance/settings/);个人 / 小团队通常用不上。
9. **更强隔离的选项:** Docker Sandboxes 把 AI 代理放进隔离环境(本地要求 Apple silicon + macOS 14+)[S34](https://docs.docker.com/ai/sandboxes/)[S35](https://docs.docker.com/ai/sandboxes/install/);其内部能否再用宿主 Docker 起库:未核实。OrbStack 文档提醒:共享内核的隔离机器不能替代完整 VM 来防御恶意代码 [S45](https://docs.orbstack.dev/architecture)。
10. **Docker Model Runner 的 API 无认证** [S33](https://docs.docker.com/ai/model-runner/)。若开启,别把端口暴露给不可信网络 / 容器。
11. **密钥。** 示例中的 `example` / `my-secret-pw` 仅限本机一次性开发库。MySQL 镜像支持 `*_FILE` 从文件读取密码,适合配合 Docker secrets [S38](https://hub.docker.com/_/mysql)。MCP Toolkit 使用 Docker Desktop 的 secrets 管理而非环境变量(`docker mcp secret`)[S2](https://github.com/docker/mcp-gateway)。

---

## 6. 来源列表

所有 URL 查阅日期:2026-10-10。

**Docker Desktop / MCP**
- [S1] Docker MCP Toolkit — https://docs.docker.com/ai/mcp-catalog-and-toolkit/toolkit/
- [S2] docker/mcp-gateway README(MIT) — https://github.com/docker/mcp-gateway
- [S3] Docker Desktop 许可 — https://docs.docker.com/subscription/desktop-license/
- [S3a] Docker 定价页(价格数字已用原始 HTML 复核) — https://www.docker.com/pricing/
- [S4] Docker Desktop Mac 安装(默认不建 docker.sock、系统要求) — https://docs.docker.com/desktop/setup/install/mac-install/
- [S5] Docker Desktop 设置(Allow the default Docker socket、Resource Saver 等) — https://docs.docker.com/desktop/settings-and-maintenance/settings/
- [S5b] 同上([S5] 页中 VMM、VirtioFS、内存默认值、Enhanced Container Isolation 条目) — https://docs.docker.com/desktop/settings-and-maintenance/settings/
- [S7] Docker MCP Catalog — https://docs.docker.com/ai/mcp-catalog-and-toolkit/catalog/
- [S8] docker/mcp-registry — https://github.com/docker/mcp-registry
- [S9] Redis MCP Server(Docker MCP 目录页) — https://hub.docker.com/mcp/server/redis/overview
- [S10] MCP Database Server(目录页) — https://hub.docker.com/mcp/server/database-server/overview
- [S11] registry 仓库中的条目:`servers/docker`、`servers/dockerhub`、`servers/database-server`(通过 GitHub API 读取;`gh api repos/docker/mcp-registry/contents/servers/<名称>/server.yaml`) — https://github.com/docker/mcp-registry/tree/main/servers
- [S31] docker/hub-mcp(Apache-2.0) — https://github.com/docker/hub-mcp
- [S33] Docker Model Runner — https://docs.docker.com/ai/model-runner/
- [S34] Docker Sandboxes — https://docs.docker.com/ai/sandboxes/
- [S35] Docker Sandboxes 安装 — https://docs.docker.com/ai/sandboxes/install/
- [S36] Docker Sandboxes 支持的代理 — https://docs.docker.com/ai/sandboxes/agents/

**社区 MCP server**
- [S12] ckreiling/mcp-server-docker(GPL-3.0) — https://github.com/ckreiling/mcp-server-docker
- [S12b] 同上 README 与源码(`docker.from_env()`、socket 挂载、安全提示);最近推送与版本用 GitHub API 查询 — https://github.com/ckreiling/mcp-server-docker
- [S13] QuantGeekDev/docker-mcp(MIT) — https://github.com/QuantGeekDev/docker-mcp
- [S13b] 同上最近推送日期(GitHub API,2024-12-14) — https://github.com/QuantGeekDev/docker-mcp

**其他运行时**
- [S6] OrbStack 定价 — https://orbstack.dev/pricing
- [S6b] OrbStack 许可条款 — https://docs.orbstack.dev/licensing
- [S16] OrbStack 文档首页 — https://docs.orbstack.dev/
- [S17] OrbStack Docker 页 — https://docs.orbstack.dev/docker/
- [S18] OrbStack 功能页 — https://docs.orbstack.dev/features
- [S18b] OrbStack 效率页 — https://docs.orbstack.dev/efficiency
- [S19b] OrbStack 基准页 — https://docs.orbstack.dev/benchmarks
- [S45] OrbStack 架构页 — https://docs.orbstack.dev/architecture
- [S14] Colima README(MIT) — https://github.com/abiosoft/colima
- [S22] Colima 仓库(许可与最新发布,GitHub API) — https://github.com/abiosoft/colima
- [S23] Podman Desktop 文档 — https://podman-desktop.io/docs/intro
- [S24a] Podman 仓库许可(GitHub API:Apache-2.0) — https://github.com/containers/podman
- [S24b] Podman Desktop 仓库许可(Apache-2.0) — https://github.com/containers/podman-desktop
- [S24c] Docker Compose 仓库许可(Apache-2.0) — https://github.com/docker/compose
- [S24d] testcontainers-java 仓库许可(MIT) — https://github.com/testcontainers/testcontainers-java
- [S24e] Lima 仓库许可(Apache-2.0) — https://github.com/lima-vm/lima
- [S26] Podman 安装(macOS) — https://podman.io/docs/installation
- [S27] podman-machine 手册(provider) — https://docs.podman.io/en/latest/markdown/podman-machine.1.html
- [S15] Rancher Desktop 文档 — https://docs.rancherdesktop.io/
- [S15b] Rancher Desktop 容器引擎偏好 — https://docs.rancherdesktop.io/ui/preferences/container-engine/general/
- [S25] rancher-sandbox/rancher-desktop(Apache-2.0) — https://github.com/rancher-sandbox/rancher-desktop
- [S28] Rancher Desktop 模拟 / VM 类型 — https://docs.rancherdesktop.io/ui/preferences/virtual-machine/emulation/

**Compose / Testcontainers / 镜像 / 网络**
- [S29] Docker Compose 概览 — https://docs.docker.com/compose/
- [S30] `docker compose up` 参考 — https://docs.docker.com/reference/cli/docker/compose/up/
- [S37] Compose 文件参考(healthcheck) — https://docs.docker.com/reference/compose-file/services/#healthcheck
- [S19] Testcontainers Java MySQL 模块 — https://java.testcontainers.org/modules/databases/mysql/
- [S20] Testcontainers JUnit 5 快速开始(Redis 示例) — https://java.testcontainers.org/quickstart/junit_5_quickstart/
- [S21] Testcontainers 支持的 Docker 环境 — https://java.testcontainers.org/supported_docker_environment/
- [S38] Docker Hub 官方镜像 mysql — https://hub.docker.com/_/mysql
- [S39] Docker Hub 官方镜像 redis — https://hub.docker.com/_/redis
- [S40a] Docker Hub API:library/mysql(标签与架构;`full_description` 为 Hub 页原文,含 License 一节;2026-10-10 查询) — https://hub.docker.com/v2/namespaces/library/repositories/mysql/tags
- [S40b] Docker Hub API:library/redis(同上,含 License 一节原文) — https://hub.docker.com/v2/namespaces/library/repositories/redis/tags
- [S41] Docker 端口发布 — https://docs.docker.com/engine/network/port-publishing/
- [S43] Docker Desktop 网络(host.docker.internal) — https://docs.docker.com/desktop/features/networking/networking-how-tos/
- [S46] Docker MCP Server(目录页;作者 Docker Inc.,工具 `docker`,镜像 `docker:cli`) — https://hub.docker.com/mcp/server/docker/overview
- [S47] `docker compose down` 参考(`-v, --volumes`) — https://docs.docker.com/reference/cli/docker/compose/down/
- [S44] Docker Engine 安全(守护进程攻击面) — https://docs.docker.com/engine/security/#docker-daemon-attack-surface

**Claude Code**
- [S32] Claude Code MCP 文档 — https://code.claude.com/docs/en/mcp
- [S42] Claude Code 沙箱文档(`docker` 不兼容、`excludedCommands`、docker.sock 风险、`allowLocalBinding`) — https://code.claude.com/docs/en/sandboxing
