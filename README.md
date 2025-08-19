# InfoClash - Clash 流量可视化分析工具

InfoClash 是一个用于收集、存储和可视化 Clash 网络连接流量数据的工具。它通过定期从 Clash API 获取连接信息，将其存入 SQLite 数据库，并提供一个 Web 界面进行数据查询、汇总和分析。

[demo](/demo.avif)

## ✨ 功能特性

- **实时流量监控**: 定期从 Clash API 同步连接数据，近乎实时地反映网络状况。
- **持久化存储**: 将流量数据存储在 SQLite 数据库中，方便长期追踪和分析。
- **数据聚合与归档**: 支持将高频、短时的连接记录按时间窗口合并，减少数据冗余，并对旧数据进行归档。
- **Web 可视化界面**: 提供一个基于 Vue 3 和 Naive UI 的现代化前端界面，用于：
    -   分页、筛选和排序连接列表。
    -   以图表形式展示按天或按小时汇总的流量数据。
    -   查看按总流量排序的主机排行榜。
- **灵活的配置**: 支持通过命令行参数、`.env` 文件或环境变量进行配置，部署方便。
- **单文件部署**: 前端应用被嵌入到后端的 Go 可执行文件中，整个应用可以作为一个单文件进行部署。

---

## 🚀 快速开始

### 1. 编译可执行文件

本项目包含一个 `build.sh` 脚本，可以自动完成前端和后端的完整构建流程。

在项目根目录下运行以下命令：

```bash
./build.sh
```

这个脚本会执行以下操作：
1.  进入 `frontend` 目录，安装 npm 依赖并构建前端静态资源。
2.  将构建好的前端文件 (`dist` 目录) 复制到 `backend` 目录。
3.  进入 `backend` 目录，编译 Go 源代码，并将前端文件嵌入到最终的可执行文件中。
4.  在项目根目录生成一个名为 `infoclash` 的可执行文件。

### 2. 配置应用程序

InfoClash 提供了多种配置方式，优先级从高到低如下：

1.  **命令行参数** (最高)
2.  **`.env` 文件**
3.  **环境变量**
4.  **默认值** (最低)

#### 通过命令行参数运行

这是最直接的运行方式。所有配置项都可以通过命令行标志指定。

```bash
./infoclash -url http://192.168.1.1:9090/connections -t YOUR_CLASH_TOKEN -p 8081
```

**常用参数:**

| 参数 | 简写 | 描述 | 默认值 |
| :--- | :--- | :--- | :--- |
| `-url` | | (必须) Clash API 的 URL | |
| `-t` | | (必须) Clash API 的 Token（secret） | |
| `-db` | | 主数据库文件的路径 | `./clash_traffic.db` |
| `-adb` | | 归档数据库文件的路径 | `./clash_traffic_archive.db` |
| `-i` | | 数据库写入间隔 (分钟) | `3` |
| `-p` | | Web 服务监听的端口 | `8081` |

使用 `-h`, `-help` 或 `--help` 查看所有参数的详细中文说明。

#### 通过 `.env` 文件配置

您可以在项目根目录的 `backend` 文件夹下创建一个 `.env` 文件来配置应用。您可以直接复制并重命名 `backend/.env.example` 文件，然后根据您的实际情况修改其中的值。这是一个示例：

```dotenv
# backend/.env

# Clash API URL
CLASH_API_URL=http://192.168.1.1:9090/connections

# Clash API Token（secret）
CLASH_API_TOKEN=YOUR_CLASH_TOKEN

# SQLite 数据库文件路径
DATABASE_PATH=./clash_traffic.db

# SQLite 归档数据库文件路径
ARCHIVE_DATABASE_PATH=./clash_traffic_archive.db

# 数据库写入间隔（分钟）
DB_WRITE_INTERVAL_MINUTES=3

# Web 服务监听端口
WEB_PORT=8081

# 域名后缀名单，用于合并相同后缀的host
HOST_SUFFIX_WHITELIST=googlevideo.com,ad.com
```

配置好后，直接运行可执行文件即可：
```bash
./infoclash
```

---

## 👨‍💻 开发文档

### 1. 启动开发服务器

为了方便开发，前端和后端可以独立运行，并启用热重载。

#### 启动后端开发服务器

后端使用 `dev` 构建标签来区分开发和生产模式。在开发模式下，后端不会嵌入前端文件，而是专注于提供 API 服务。

在启动后端之前，您需要配置本地的开发环境变量。

1.  **创建 .env 文件**: 将 `backend/.env.example` 文件复制一份并重命名为 `backend/.env`。
    ```bash
    cp backend/.env.example backend/.env
    ```
2.  **修改配置**: 打开 `backend/.env` 文件，根据您的本地 Clash API 地址和 Token 修改其中的值。

完成配置后，即可启动后端开发服务器：
```bash
# 进入后端目录
cd backend

# 使用 dev 标签运行
go run -tags dev .
```

默认情况下，后端 API 服务器会运行在 `http://localhost:8081`。

#### 启动前端开发服务器

前端使用 Vite 作为开发服务器，它提供了极速的热重载和丰富的插件生态。

前端开发服务器需要知道后端 API 的地址，以便正确代理 API 请求。

1.  **创建 .env 文件**: 将 `frontend/.env.example` 文件复制一份并重命名为 `frontend/.env`。
    ```bash
    cp frontend/.env.example frontend/.env
    ```
2.  **修改配置 (如果需要)**: 打开 `frontend/.env` 文件。如果您的后端服务**没有**运行在默认的 `http://localhost:8081`，请修改 `VITE_API_BASE_URL` 的值为您后端服务的实际地址。

完成配置后，即可安装依赖并启动前端开发服务器：
```bash
# 进入前端目录
cd frontend

# 安装依赖
npm install

# 启动开发服务器
npm run dev
```

Vite 会启动一个开发服务器，通常在 `http://localhost:5173`。它会自动将 `/api` 请求代理到您在 `.env` 文件中配置的后端地址。

现在，您可以在浏览器中打开 `http://localhost:5173` 来访问前端页面，对代码的任何修改都会立即反映在浏览器中。

### 2. API 设计

所有 API 的基础路径为 `/api`。

#### `GET /api/connections`
获取连接的详细记录，支持筛选、排序和分页。
*详细参数和响应请参考 [API_DESIGN.md](API_DESIGN.md)*

#### `POST /api/connections/merge`
合并指定时间范围内的连接记录。
*详细参数和响应请参考 [API_DESIGN.md](API_DESIGN.md)*

#### `POST /api/connections/replace-host`
替换具有特定域名后缀的主机名。
*详细参数和响应请参考 [API_DESIGN.md](API_DESIGN.md)*

#### `GET /api/summary/traffic`
获取按时间粒度（天或小时）分组的流量汇总数据。
*详细参数和响应请参考 [API_DESIGN.md](API_DESIGN.md)*

#### `GET /api/summary/hosts`
获取按总流量排序的主机排名。
*详细参数和响应请参考 [API_DESIGN.md](API_DESIGN.md)*

#### `GET /api/hosts`
获取所有不重复的主机名列表。
*详细参数和响应请参考 [API_DESIGN.md](API_DESIGN.md)*

#### `GET /api/chains`
获取所有不重复的代理链名称列表。
*详细参数和响应请参考 [API_DESIGN.md](API_DESIGN.md)*

### 3. 数据库结构

项目使用 SQLite 存储数据，包含两个主要的表。

#### 表: `connections`
该表用于存储从 Clash API 获取的每个连接的详细流量信息。
*详细结构请参考 [DATABASE_SCHEMA.md](DATABASE_SCHEMA.md)*

#### 表: `connections_archive`
该表用于存储已关闭或已合并连接的最终流量信息。
*详细结构请参考 [DATABASE_SCHEMA.md](DATABASE_SCHEMA.md)*