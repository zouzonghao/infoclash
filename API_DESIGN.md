# API 设计文档

本文档定义了 `infoclash` 前端与后端交互的 RESTful API。

## 基础 URL

所有 API 的基础路径为 `/api`。

---

## 1. 连接记录 (Connections)

### `GET /api/connections`

获取连接的详细记录，支持筛选、排序和分页。

#### 查询参数 (Query Parameters)

| 参数 | 类型 | 可选 | 描述 | 默认值 | 示例 |
| :--- | :--- | :--- | :--- | :--- | :--- |
| `page` | `integer` | 是 | 请求的页码，从 1 开始。 | `1` | `?page=2` |
| `pageSize` | `integer` | 是 | 每页返回的记录数。 | `20` | `?pageSize=50` |
| `host` | `string` | 是 | 按主机名进行模糊搜索 (`LIKE %host%`)。 | | `?host=cloudflare` |
| `sourceIP` | `string` | 是 | 按源 IP 地址进行模糊搜索 (`LIKE %sourceIP%`)。 | | `?sourceIP=192.168` |
| `chain` | `string` | 是 | 按代理链名称进行精确匹配。 | | `?chain=DIRECT` |
| `startDate` | `integer` | 是 | 查询的开始时间 (Unix 时间戳, 秒)。 | | `?startDate=1672531200` |
| `endDate` | `integer` | 是 | 查询的结束时间 (Unix 时间戳, 秒)。 | | `?endDate=1675209600` |
| `sortBy` | `string` | 是 | 排序字段。可选值: `upload`, `download`, `start`, `metadata.host`, `metadata.sourceIP`。 | `start` | `?sortBy=download` |
| `sortOrder` | `string` | 是 | 排序顺序。可选值: `asc`, `desc`。 | `desc` | `?sortOrder=asc` |

#### 成功响应 (200 OK)

```json
{
  "total": 125,
  "page": 1,
  "pageSize": 20,
  "totalPages": 7,
  "data": [
    {
      "host": "speed.cloudflare.com",
      "sourceIP": "192.168.2.95",
      "upload": 10240,
      "download": 512000,
      "start": "2023-01-01T12:00:00Z",
      "chains": ["🚀 节点选择"]
    }
  ]
}
```

---

### `POST /api/connections/merge`

合并指定时间范围内的连接记录，将短时、高频的连接聚合成一个总记录，以减少数据库中的数据量。

#### 请求体 (Request Body)

```json
{
  "startDate": 1672531200,
  "endDate": 1675209600,
  "interval": 5,
  "deleteSource": true
}
```

| 字段 | 类型 | 必须 | 描述 |
| :--- | :--- | :--- | :--- |
| `startDate` | `integer` | 是 | 合并范围的开始时间 (Unix 时间戳, 秒)。 |
| `endDate` | `integer` | 是 | 合并范围的结束时间 (Unix 时间戳, 秒)。 |
| `interval` | `integer` | 是 | 合并的时间窗口大小，单位为分钟。例如，`5` 表示将每 5 分钟内的相同主机的记录合并为一条。 |

#### 成功响应 (200 OK)

```json
{
  "message": "合并成功"
}
```

---

### `POST /api/connections/replace-host`

将具有特定域名后缀的主机名替换为其根域名。例如，将 `v1.example.com` 和 `v2.example.com` 都替换为 `example.com`。

#### 请求体 (Request Body)

```json
{
  "domainSuffix": "example.com"
}
```

| 字段 | 类型 | 必须 | 描述 |
| :--- | :--- | :--- | :--- |
| `domainSuffix` | `string` | 是 | 要替换的目标域名。所有以 `.` + `domainSuffix` 结尾或完全等于 `domainSuffix` 的主机名都将被更新。 |

#### 成功响应 (200 OK)

```json
{
  "message": "替换成功",
  "rowsAffected": 15
}
```

---

## 2. 流量汇总 (Summary)

### `GET /api/summary/traffic`

获取按时间粒度（天或小时）分组的流量汇总数据，用于绘制时间序列图表。

#### 查询参数 (Query Parameters)

| 参数 | 类型 | 可选 | 描述 | 默认值 | 示例 |
| :--- | :--- | :--- | :--- | :--- | :--- |
| `granularity` | `string` | 是 | 时间粒度。可选值: `day`, `hour`。 | `day` | `?granularity=hour` |
| `host` | `string` | 是 | 按特定主机名进行筛选。 | | `?host=speed.cloudflare.com` |
| `startDate` | `integer` | 是 | 查询的开始时间 (Unix 时间戳, 秒)。 | | `?startDate=1672531200` |
| `endDate` | `integer` | 是 | 查询的结束时间 (Unix 时间戳, 秒)。 | | `?endDate=1675209600` |

#### 成功响应 (200 OK)

```json
[
  {
    "time": "2023-01-01 00:00:00",
    "upload": 1048576,
    "download": 20971520
  },
  {
    "time": "2023-01-02 00:00:00",
    "upload": 2097152,
    "download": 31457280
  }
]
```

---

### `GET /api/summary/hosts`

获取按总流量（上传 + 下载）排序的主机排名。

#### 查询参数 (Query Parameters)

| 参数 | 类型 | 可选 | 描述 | 默认值 | 示例 |
| :--- | :--- | :--- | :--- | :--- | :--- |
| `limit` | `integer` | 是 | 返回的排名数量。 | `10` | `?limit=20` |
| `startDate` | `integer` | 是 | 查询的开始时间 (Unix 时间戳, 秒)。 | | `?startDate=1672531200` |
| `endDate` | `integer` | 是 | 查询的结束时间 (Unix 时间戳, 秒)。 | | `?endDate=1675209600` |

#### 成功响应 (200 OK)

```json
[
  {
    "host": "speed.cloudflare.com",
    "upload": 1073741824,
    "download": 53687091200,
    "total": 54760833024
  },
  {
    "host": "api.google.com",
    "upload": 5242880,
    "download": 104857600,
    "total": 110100480
  }
]
```

---

## 3. 辅助接口 (Helpers)

### `GET /api/hosts`

获取数据库中所有不重复的主机名列表，用于筛选器下拉菜单。

#### 查询参数 (Query Parameters)

无。

#### 成功响应 (200 OK)

```json
[
  "speed.cloudflare.com",
  "api.google.com",
  "v2ex.com"
]
```

---

### `GET /api/chains`

获取数据库中所有不重复的代理链名称列表，用于筛选器下拉菜单。

#### 查询参数 (Query Parameters)

无。

#### 成功响应 (200 OK)

```json
[
  "DIRECT",
  "PROXY",
  "🚀 节点选择"
]