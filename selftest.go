package main

import "fmt"

// selftest 在临时内存状态上验证核心流程，任何失败都返回错误且不改变真实状态。
func selftest() (Response, error) {
	st := newState()
	f := func(ok bool, msg string) {
		if !ok {
			panic("selftest failed: " + msg)
		}
	}
	strp := func(s string) *string { return &s }
	fp := func(v float64) *float64 { return &v }
	ip := func(v int) *int { return &v }

	// 登记设备。
	if _, err := handle(st, Command{
		Type: "register_equipment", EquipmentID: strp("eq1"),
		InitialReading: fp(100), InitialDate: strp("2026-01-01"),
	}); err != nil {
		return nil, err
	}
	// 重复设备应失败。
	if _, err := handle(st, Command{
		Type: "register_equipment", EquipmentID: strp("eq1"),
		InitialReading: fp(0), InitialDate: strp("2026-01-01"),
	}); err == nil {
		return nil, fmt.Errorf("duplicate equipment not rejected")
	}
	// 非法日期（2 月 30 日）应失败。
	if _, err := handle(st, Command{
		Type: "register_equipment", EquipmentID: strp("eq2"),
		InitialReading: fp(0), InitialDate: strp("2026-02-30"),
	}); err == nil {
		return nil, fmt.Errorf("nonexistent date not rejected")
	}
	// 建立日历 + 运行量双周期项目。
	if _, err := handle(st, Command{
		Type: "create_item", EquipmentID: strp("eq1"), ItemID: strp("oil"),
		DaysInterval: ip(30), UsageInterval: fp(500),
	}); err != nil {
		return nil, err
	}
	// 无周期应失败。
	if _, err := handle(st, Command{
		Type: "create_item", EquipmentID: strp("eq1"), ItemID: strp("bad"),
	}); err == nil {
		return nil, fmt.Errorf("item without interval not rejected")
	}
	// 读数日期必须严格递增、读数不下降。
	if _, err := handle(st, Command{
		Type: "add_reading", EquipmentID: strp("eq1"),
		Date: strp("2026-01-01"), Reading: fp(200),
	}); err == nil {
		return nil, fmt.Errorf("non-increasing date not rejected")
	}
	if _, err := handle(st, Command{
		Type: "add_reading", EquipmentID: strp("eq1"),
		Date: strp("2026-01-10"), Reading: fp(50),
	}); err == nil {
		return nil, fmt.Errorf("decreasing reading not rejected")
	}
	// 有效读数：运行量到期（601-600=1 <= 0? 601-600=1 未到期），推到 600 增量。
	if _, err := handle(st, Command{
		Type: "add_reading", EquipmentID: strp("eq1"),
		Date: strp("2026-01-10"), Reading: fp(600),
	}); err != nil {
		return nil, err
	}
	// 完成保养重置基准。
	if _, err := handle(st, Command{
		Type: "complete_maintenance", EquipmentID: strp("eq1"), ItemID: strp("oil"),
	}); err != nil {
		return nil, err
	}
	resp, err := handle(st, Command{
		Type: "maintenance_history", EquipmentID: strp("eq1"), ItemID: strp("oil"),
	})
	if err != nil {
		return nil, err
	}
	hist := resp["history"].(map[string][]CompletionRecord)
	f(len(hist["oil"]) == 1, "history should contain one completion")
	f(hist["oil"][0].Date == "2026-01-10", "completion should use latest date")

	// 查询计划：重置后运行量剩余 500，未到期；查询日期早于读数日应失败。
	if _, err := handle(st, Command{
		Type: "query_plan", Date: strp("2026-01-09"),
	}); err == nil {
		return nil, fmt.Errorf("query date before latest reading not rejected")
	}
	resp, err = handle(st, Command{Type: "query_plan", Date: strp("2026-01-11")})
	if err != nil {
		return nil, err
	}
	items := resp["items"].([]PlanItem)
	f(len(items) == 1, "expected one plan item")
	f(!items[0].Due, "item should not be due right after reset")
	f(items[0].DaysRemaining == 29, "days remaining should be 29")

	// 推进到日历到期日并再次查询。
	if _, err := handle(st, Command{
		Type: "add_reading", EquipmentID: strp("eq1"),
		Date: strp("2026-02-10"), Reading: fp(600),
	}); err != nil {
		return nil, err
	}
	resp, err = handle(st, Command{Type: "query_plan", Date: strp("2026-02-10")})
	if err != nil {
		return nil, err
	}
	items = resp["items"].([]PlanItem)
	f(items[0].Due && items[0].DaysDue, "item should be due by calendar after 30 days")

	return Response{"ok": true, "selftest": "passed", "checks": 12}, nil
}
