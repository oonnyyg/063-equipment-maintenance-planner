# 设备保养计划工具

将日历周期与累计运行量合并为可追溯的到期清单。仅使用 Go 标准库。

## 运行

```sh
go run . --state 状态文件路径
```

程序从标准输入逐行读取 JSON 命令，逐行输出 JSON 响应。状态在每条变更命令后
原子化保存到 `--state` 指定的文件，退出重启后数据保留。状态文件不存在时从空
台账开始。

运行自测：

```sh
go test ./...
```

## 命令格式

每行一个 JSON 对象，`cmd` 字段指定命令。响应为一行 JSON：
成功 `{"ok":true,"cmd":"...",...}`，失败 `{"ok":false,"cmd":"...","error":"第 N 行: ..."}`。
任何非法命令都不会改变状态，后续命令继续处理。

### register_device — 登记设备

```json
{"cmd":"register_device","device":"pump1","reading":100,"date":"2026-01-01"}
```

- `reading` 为初始累计运行量，必须有限非负。
- `date` 为 `YYYY-MM-DD`，必须是真实存在的日期。
- 设备标识重复时报错。

### add_item — 登记保养项目

```json
{"cmd":"add_item","device":"pump1","item":"oil","days":30,"usage":500}
```

- 至少配置一种周期：`days`（正整数日间隔）或 `usage`（正数运行量间隔），可同时配置。
- 以设备当前最新读数及其日期为基准建立项目。
- 项目标识在设备内唯一，各项目独立累计。

### add_reading — 追加读数

```json
{"cmd":"add_reading","device":"pump1","reading":650,"date":"2026-01-20"}
```

- 日期必须严格晚于该设备最新读数日期，读数不得下降。

### complete — 完成保养

```json
{"cmd":"complete","device":"pump1","item":"oil"}
```

- 仅重置指定项目：以设备最新读数及其日期作为新基准。
- 完成记录（日期、读数）保留在项目历史中。

### plan — 查询到期清单

```json
{"cmd":"plan","date":"2026-01-20"}
```

- 查询日期不得早于台账内任何读数日。
- 使用各设备最新读数计算，不预测运行量。
- 每条结果列出启用周期的下次阈值与剩余额度：
  - 日历周期：`next_date`、`remaining_days`
  - 运行量周期：`next_reading`、`remaining_usage`
- 任一剩余额度小于等于零即 `due:true`。
- 结果到期项优先，其余按设备、项目标识排序。

### history — 查询保养历史

```json
{"cmd":"history","device":"pump1","item":"oil"}
```

- `device`、`item` 均可省略表示不过滤；指定时必须存在。
- 返回按日期、设备、项目排序的完成记录。

## 完整示例

```sh
$ go run . --state demo.json
{"cmd":"register_device","device":"pump1","reading":100,"date":"2026-01-01"}
{"ok":true,"cmd":"register_device"}
{"cmd":"add_item","device":"pump1","item":"oil","days":30,"usage":500}
{"ok":true,"cmd":"add_item"}
{"cmd":"add_reading","device":"pump1","reading":650,"date":"2026-01-20"}
{"ok":true,"cmd":"add_reading"}
{"cmd":"plan","date":"2026-01-20"}
{"ok":true,"cmd":"plan","plan":[{"device":"pump1","item":"oil","due":true,"next_date":"2026-01-31","remaining_days":11,"next_reading":600,"remaining_usage":-50}]}
{"cmd":"complete","device":"pump1","item":"oil"}
{"ok":true,"cmd":"complete"}
{"cmd":"history"}
{"ok":true,"cmd":"history","history":[{"device":"pump1","item":"oil","date":"2026-01-20","reading":650}]}
{"cmd":"plan","date":"2026-01-20"}
{"ok":true,"cmd":"plan","plan":[{"device":"pump1","item":"oil","due":false,"next_date":"2026-02-19","remaining_days":30,"next_reading":1150,"remaining_usage":500}]}
```

退出后重新以同一 `--state` 路径启动，以上数据仍然可用。
