# 数据库结构文档

本文档详细描述了 `infoclash` 项目用于存储 Clash 连接信息的 SQLite 数据库结构。

## 表: `connections`

该表用于存储从 Clash API 获取的每个连接的详细流量信息。

### 表结构

| 字段名 (Field) | 数据类型 (Type) | 约束 (Constraints) | 描述 (Description) |
| :--- | :--- | :--- | :--- |
| `id` | `TEXT` | `NOT NULL`, `PRIMARY KEY` | 连接的唯一标识符 (UUID)，来自 Clash API。作为主键。 |
| `sourceIP` | `TEXT` | | 连接的源 IP 地址。例如: `192.168.2.95`。 |
| `host` | `TEXT` | | 连接的目标主机名。例如: `speed.cloudflare.com`。 |
| `upload` | `INTEGER` | | 该连接自建立以来的总上传流量，单位为字节 (Bytes)。 |
| `download` | `INTEGER` | | 该连接自建立以来的总下载流量，单位为字节 (Bytes)。 |
| `start` | `INTEGER` | | 连接建立的 Unix 时间戳 (秒)。 |
| `chain` | `TEXT` | | Clash 中该连接所经过的代理链中的最后一个节点的名称。例如: `🚀 节点选择`。 |

### SQL 创建语句

```sql
CREATE TABLE IF NOT EXISTS connections (
    "id" TEXT NOT NULL PRIMARY KEY,
    "sourceIP" TEXT,
    "host" TEXT,
    "upload" INTEGER,
    "download" INTEGER,
    "start" INTEGER,
    "chain" TEXT
);
```

### 使用说明

-   **主键**：`id` 字段是唯一的，可以用来区分不同的连接。程序使用 `INSERT ... ON CONFLICT DO UPDATE` (Upsert) 逻辑，这意味着：
    -   如果数据库中已存在相同 `id` 的记录，程序将更新该记录的 `upload` 和 `download` 字段。
    -   如果 `id` 不存在，则会插入一条新记录。
-   **流量单位**：`upload` 和 `download` 字段的单位是字节。在进行分析时，您可能需要将其转换为 KB, MB 或 GB (例如, `download / 1024.0 / 1024.0` 得到 MB)。
-   **时间戳**：`start` 字段存储的是标准的 Unix 时间戳 (秒)。您可以使用任何编程语言或数据库函数轻松地将其转换为人类可读的日期时间格式。


## 表: `connections_archive`

该表用于存储已关闭连接的最终流量信息。当一个连接在 Clash 中关闭后，它的最终状态会被从 `connections` 表移动到此表中进行归档。

### 表结构

| 字段名 (Field) | 数据类型 (Type) | 约束 (Constraints) | 描述 (Description) |
| :--- | :--- | :--- | :--- |
| `id` | `TEXT` | `NOT NULL` | 连接的唯一标识符 (UUID)。与 `connections` 表中的 `id` 对应。 |
| `sourceIP` | `TEXT` | | 连接的源 IP 地址。 |
| `host` | `TEXT` | | 连接的目标主机名。 |
| `upload` | `INTEGER` | | 连接关闭时的总上传流量，单位为字节 (Bytes)。 |
| `download` | `INTEGER` | | 连接关闭时的总下载流量，单位为字节 (Bytes)。 |
| `start` | `INTEGER` | | 连接建立的 Unix 时间戳 (秒)。 |
| `chain` | `TEXT` | | Clash 中该连接所经过的代理链。 |
| `archived_at` | `INTEGER` | | 记录归档时的 Unix 时间戳 (秒)。 |

### SQL 创建语句

```sql
CREATE TABLE IF NOT EXISTS connections_archive (
    "id" TEXT NOT NULL,
    "sourceIP" TEXT,
    "host" TEXT,
    "upload" INTEGER,
    "download" INTEGER,
    "start" INTEGER,
    "chain" TEXT,
    "archived_at" INTEGER
);
```

### 使用说明

-   **数据来源**：此表中的数据来自于 `connections` 表。当一个连接不再活跃（即从 Clash API 的连接列表中消失），该连接的记录将从 `connections` 表中删除，并在此处创建一条归档记录。
-   **主键**：此表没有显式的主键。`id` 字段不是唯一的，因为同一个连接可能会由于程序重启等原因被多次归档。分析数据时，可以考虑使用 `id` 和 `archived_at` 的组合来识别特定的归档事件。
-   **时间戳**：`archived_at` 字段记录了数据归档的时间，可用于按时间范围查询历史流量数据。