# code-seek

一个轻量级的代码搜索 HTTP 服务，提供类 fzf 的模糊文件搜索和基于 tree-sitter 的符号搜索（函数、类、类型等），专为编辑器/工具集成设计。

## 特性

- **模糊文件搜索** — 类 fzf 的路径模糊匹配，按相关度排序
- **符号搜索** — 基于 [tree-sitter](https://github.com/smacker/go-tree-sitter) 解析源码，支持搜索函数、方法、类、类型等符号
- **并发取消** — 通过 `session_id` 自动取消同一会话的旧请求，适合编辑器输入联想场景
- **多语言支持** — Go、Python、JavaScript/JSX、TypeScript/TSX、Ruby、Java、Rust、C/C++

## 安装

需要 Go 1.21+。

```bash
git clone <repo>
cd code-seek
go build -o code-seek .
```

## 启动

```bash
# 默认监听 :15654
./code-seek

# 指定端口
./code-seek -addr :8080
```

## 使用

### 搜索文件

```bash
curl "http://localhost:15654/search?work_dir=/your/project&query=handler"
```

```json
{
  "results": [
    {"file": "handler/search.go"},
    {"file": "handler/content.go"}
  ]
}
```

加上 `details=true` 可以同时返回行数和文件大小：

```bash
curl "http://localhost:15654/search?work_dir=/your/project&query=handler&details=true"
```

### 搜索符号

使用 `文件模式:符号模式` 语法：

```bash
# 在 handler 相关文件中搜索名含 Search 的符号
curl "http://localhost:15654/search?work_dir=/your/project&query=handler:Search"

# 在全部文件中搜索符号（文件模式留空）
curl "http://localhost:15654/search?work_dir=/your/project&query=:Search"
```

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

### 取消旧请求（编辑器集成）

传入 `session_id`，同一会话的新请求会自动取消上一次未完成的搜索：

```bash
curl "http://localhost:15654/search?work_dir=/your/project&query=han&session_id=editor-1"
# 用户继续输入，发起新请求，旧请求自动取消
curl "http://localhost:15654/search?work_dir=/your/project&query=handler&session_id=editor-1"
```

### 读取文件内容

支持同时读取多个文件，可指定行范围：

```bash
curl -X POST http://localhost:15654/content \
  -H "Content-Type: application/json" \
  -d '{
    "work_dir": "/your/project",
    "files": [
      {"path": "main.go"},
      {"path": "handler/search.go", "ranges": [[1, 15], [25, 28]]}
    ]
  }'
```

```json
{
  "files": [
    {
      "path": "main.go",
      "total_lines": 25,
      "size": 320,
      "segments": [{"start_line": 1, "end_line": 25, "content": "..."}]
    },
    {
      "path": "handler/search.go",
      "total_lines": 28,
      "size": 512,
      "segments": [
        {"start_line": 1, "end_line": 15, "content": "..."},
        {"start_line": 25, "end_line": 28, "content": "..."}
      ]
    }
  ]
}
```

### 健康检查

```bash
curl http://localhost:15654/health
# 响应: ok
```

## API 文档

完整的接口说明请参阅 [API.md](API.md)。

## License

[MIT](LICENSE)
