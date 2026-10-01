# 设备保养计划工具（equipment-maintenance-planner）

纯标准库 Go 程序，将**日历周期**与**累计运行量**两类保养周期合并，生成可追溯的到期清单。状态持久化到 JSON 文件，退出重启后保留数据。

## 运行

```bash
go run . --state ./state.json
```

- 标准输入**逐行**读取 JSON 命令，每行一条。
- 标准输出**逐行**输出 JSON 结果。
- 每条成功命令执行后立即持久化；非法命令返回错误行，**不改变状态**，后续命令继续处理。

成功响应含 `"ok": true`；错误响应形如：

```json
{"ok":false,"line":7,"error":"query date 2025-12-31 is before ...","command":"query_plan"}
```

`line` 为输入行号，`command` 为命令类型，便于定位。

## 通用规则

- 日期一律 `YYYY-MM-DD`，按自然日计算；不存在的日期（如 `2026-02-30`）拒绝。
- 读数为有限非负数（拒绝负数、`NaN`、`Infinity`）。
- 同一设备的读数日期必须**严格递增**，读数**不得下降**。
- 项目至少配置一种周期：`days_interval`（正整数，按天）或 `usage_interval`（正数，按运行量）。
- 建立项目时以设备当前最新读数/日期为基准；项目标识在设备内唯一，各项目独立累计。
- 完成保养只重置指定项目，以设备**最新**读数与日期为新基准，并追加历史记录。
- 查询使用各设备最新读数，**不预测**运行量；查询日期不得早于任何设备的最新读数日。
- 任一启用周期的剩余额度 `<= 0` 即到期。结果到期项优先，再按设备标识、项目标识排序。

## 命令格式

| type | 必填字段 | 说明 |
|---|---|---|
| `register_equipment` | `equipment_id`,`initial_reading`,`initial_date` | 登记设备及初始读数 |
| `create_item` | `equipment_id`,`item_id`，及 `days_interval` 和/或 `usage_interval` | 建立保养项目 |
| `add_reading` | `equipment_id`,`date`,`reading` | 追加一条读数 |
| `complete_maintenance` | `equipment_id`,`item_id` | 完成保养，重置该项目基准 |
| `query_plan` | `date` | 查询指定日期的到期计划 |
| `maintenance_history` | `equipment_id`，可选 `item_id` | 查询保养完成历史 |
| `selftest` | 无 | 内置自测，不改动磁盘状态 |

`query_plan` 每个项目返回：

- `due` / `days_due` / `usage_due`：到期标志；
- `next_days_date`、`days_remaining`：日历周期下次阈值日与剩余天数；
- `next_usage_threshold`、`usage_remaining`：运行量周期下次阈值与剩余额度（可负）。

## 完整操作示例

输入：

```json
{"type":"register_equipment","equipment_id":"crane","initial_reading":1000,"initial_date":"2026-01-01"}
{"type":"create_item","equipment_id":"crane","item_id":"oil","days_interval":30,"usage_interval":500}
{"type":"create_item","equipment_id":"crane","item_id":"belt","usage_interval":100}
{"type":"add_reading","equipment_id":"crane","date":"2026-01-05","reading":1120}
{"type":"query_plan","date":"2026-01-05"}
{"type":"complete_maintenance","equipment_id":"crane","item_id":"belt"}
{"type":"maintenance_history","equipment_id":"crane"}
{"type":"selftest"}
```

输出（节选）：

```json
{"equipment_id":"crane","ok":true}
{"equipment_id":"crane","item_id":"oil","ok":true}
{"equipment_id":"crane","item_id":"belt","ok":true}
{"equipment_id":"crane","latest_date":"2026-01-05","latest_reading":1120,"ok":true}
{"date":"2026-01-05","items":[{"equipment_id":"crane","item_id":"belt","due":true,"usage_due":true,"next_usage_threshold":1100,"usage_remaining":-20},{"equipment_id":"crane","item_id":"oil","due":false,"next_days_date":"2026-01-31","days_remaining":26,"next_usage_threshold":1500,"usage_remaining":380}],"ok":true}
{"equipment_id":"crane","item_id":"belt","base_date":"2026-01-05","base_usage":1120,"ok":true}
{"equipment_id":"crane","history":{"belt":[{"date":"2026-01-05","usage":1120}],"oil":[]},"ok":true}
{"ok":true,"selftest":"passed"}
```

PowerShell 下可直接管道喂入：

```powershell
Get-Content commands.txt | go run . --state ./state.json
```

## 测试

```bash
go test ./...        # 单元测试（含日期校验、自测、错误不改变状态）
'{"type":"selftest"}' | go run . --state selftest.json
```
