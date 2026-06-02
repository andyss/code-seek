# code-seek API 文档

code-seek 是一个轻量级代码搜索服务，提供文件/符号模糊搜索和文件内容读取功能。

**默认监听地址：** `:15654`（可通过 `-addr` 启动参数修改）

---

## 接口列表

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/search` | 模糊搜索文件或符号 |
| POST | `/content` | 读取文件内容（支持行范围） |
| GET | `/health` | 健康检查 |

---

## GET /search

在指定工作目录下进行模糊搜索，支持文件搜索和符号搜索两种模式。

### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `work_dir` | string | 是 | 搜索的根目录路径 |
| `query` | string | 否 | 搜索关键词，见下方查询语法 |
| `session_id` | string | 否 | 会话 ID，同一会话的新请求会自动取消上一次未完成的搜索 |
| `details` | string | 否 | 为 `true` 或 `1` 时，文件模式结果附带行数和文件大小 |

### 查询语法

- **文件模式**：`query` 不含 `:` 时，对文件路径进行模糊匹配
  - 示例：`handler` → 匹配所有路径中含 `handler` 的文件
- **符号模式**：`query` 含 `:` 时，格式为 `<文件模式>:<符号模式>`，先过滤文件再匹配符号
  - 示例：`handler:Search` → 在 `handler` 相关文件中搜索名为 `Search` 的函数/类型等
  - 文件模式或符号模式均可为空，例如 `:Search` 表示在全部文件中搜索符号 `Search`
- `query` 为空时返回全部文件（最多 500 条）

模糊匹配规则：pattern 中所有字符须在目标字符串中按顺序出现（大小写不敏感）；词边界（`/` `_` `-` `.`）和连续匹配字符会提升评分，结果按评分降序排列。

### 支持符号解析的语言

Go、Python、JavaScript/JSX、TypeScript/TSX、Ruby、Java、Rust、C/C++

### 响应

```json
{
  "results": [
    {
      "file": "handler/search.go",
      "lines": 28,
      "size": 512,
      "line": 10,
      "end_line": 28,
      "symbol": "Search",
      "kind": "function"
    }
  ]
}
```

#### Result 字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| `file` | string | 相对于 `work_dir` 的文件路径（使用 `/` 分隔） |
| `lines` | int | 文件总行数（符号模式或 `details=true` 时存在） |
| `size` | int | 文件字节数（符号模式或 `details=true` 时存在） |
| `line` | int | 符号起始行（符号模式专用，1-based） |
| `end_line` | int | 符号结束行（符号模式专用，1-based，含） |
| `symbol` | string | 符号名称（符号模式专用） |
| `kind` | string | 符号类型：`function` / `method` / `class` / `type` / `module` |

### 错误响应

| HTTP 状态码 | 说明 |
|-------------|------|
| 400 | `work_dir` 参数缺失 |

```json
{"error": "work_dir is required"}
```

### 示例

```bash
# 搜索文件名含 "search" 的文件
curl "http://localhost:15654/search?work_dir=/path/to/project&query=search"

# 搜索文件名含 "handler" 的文件，并附带行数/大小
curl "http://localhost:15654/search?work_dir=/path/to/project&query=handler&details=true"

# 在 handler 相关文件中搜索 Search 符号
curl "http://localhost:15654/search?work_dir=/path/to/project&query=handler:Search"

# 在所有文件中搜索 Search 符号，使用 session_id 取消旧请求
curl "http://localhost:15654/search?work_dir=/path/to/project&query=:Search&session_id=sess-abc"
```

---

## POST /content

读取指定文件的内容，支持同时读取多个文件，每个文件可指定多个行范围。

### 请求

**Content-Type:** `application/json`

```json
{
  "work_dir": "/path/to/project",
  "files": [
    {
      "path": "handler/search.go",
      "ranges": [[1, 10], [20, 28]]
    },
    {
      "path": "main.go"
    }
  ]
}
```

#### 请求体字段

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `work_dir` | string | 否 | 工作目录，相对路径的基准；绝对路径文件可省略 |
| `files` | FileRequest[] | 是 | 要读取的文件列表 |

#### FileRequest 字段

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `path` | string | 是 | 文件路径（相对路径相对于 `work_dir`；绝对路径直接使用） |
| `ranges` | \[\[int, int\]\] | 否 | 要读取的行范围数组，1-based 闭区间，如 `[[1,10],[20,30]]`；省略则返回整个文件 |

### 响应

```json
{
  "files": [
    {
      "path": "handler/search.go",
      "total_lines": 28,
      "size": 512,
      "segments": [
        {
          "start_line": 1,
          "end_line": 10,
          "content": "package handler\n..."
        },
        {
          "start_line": 20,
          "end_line": 28,
          "content": "..."
        }
      ]
    },
    {
      "path": "main.go",
      "total_lines": 25,
      "size": 320,
      "segments": [
        {
          "start_line": 1,
          "end_line": 25,
          "content": "package main\n..."
        }
      ]
    }
  ]
}
```

#### FileContent 字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| `path` | string | 请求中的文件路径 |
| `total_lines` | int | 文件总行数 |
| `size` | int | 文件字节数 |
| `segments` | LineSegment[] | 内容片段列表 |
| `error` | string | 读取失败时的错误信息（存在时其他字段可能为零值） |

#### LineSegment 字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| `start_line` | int | 片段起始行（1-based） |
| `end_line` | int | 片段结束行（1-based，含） |
| `content` | string | 该范围的原始文本内容 |

### 安全限制

- 相对路径会被限制在 `work_dir` 范围内，尝试访问 `work_dir` 以外的路径将返回 `"path outside work_dir"` 错误。

### 错误响应

| HTTP 状态码 | 说明 |
|-------------|------|
| 405 | 请求方法不是 POST |
| 400 | 请求体不是合法 JSON |

```json
{"error": "POST required"}
{"error": "invalid JSON body"}
```

单个文件读取失败时不影响其他文件，错误信息记录在对应 `FileContent.error` 字段中：

```json
{
  "files": [
    {
      "path": "not_found.go",
      "total_lines": 0,
      "size": 0,
      "segments": null,
      "error": "open /path/to/not_found.go: no such file or directory"
    }
  ]
}
```

### 示例

```bash
# 读取单个文件全部内容
curl -X POST http://localhost:15654/content \
  -H "Content-Type: application/json" \
  -d '{"work_dir":"/path/to/project","files":[{"path":"main.go"}]}'

# 读取多个文件，指定行范围
curl -X POST http://localhost:15654/content \
  -H "Content-Type: application/json" \
  -d '{
    "work_dir": "/path/to/project",
    "files": [
      {"path": "handler/search.go", "ranges": [[1, 15]]},
      {"path": "handler/content.go", "ranges": [[1, 10], [45, 68]]}
    ]
  }'
```

---

## GET /health

健康检查接口。

### 响应

- **HTTP 200** `ok`

### 示例

```bash
curl http://localhost:15654/health
# 响应: ok
```

---

## 通用说明

- 搜索结果上限为 **500 条**，按模糊匹配评分降序排列
- 搜索会自动跳过以下目录：`node_modules`、`vendor`、`.git`、`.svn`、`__pycache__`、`dist`、`build`、`target`、`.build`、`.gradle` 及所有以 `.` 开头的隐藏目录
- 所有响应均为 `Content-Type: application/json`（`/health` 除外）
