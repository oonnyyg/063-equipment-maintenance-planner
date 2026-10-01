package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func mustOK(t *testing.T, st *State, jsonLine string) Response {
	t.Helper()
	req := parseReq(t, jsonLine)
	resp, _ := dispatch(st, &req)
	if !resp.OK {
		t.Fatalf("命令失败: %s -> %s", jsonLine, resp.Error)
	}
	return resp
}

func mustFail(t *testing.T, st *State, jsonLine string) Response {
	t.Helper()
	req := parseReq(t, jsonLine)
	resp, _ := dispatch(st, &req)
	if resp.OK {
		t.Fatalf("命令应失败但成功: %s", jsonLine)
	}
	return resp
}

func parseReq(t *testing.T, s string) Request {
	t.Helper()
	var req Request
	if err := json.Unmarshal([]byte(s), &req); err != nil {
		t.Fatal(err)
	}
	return req
}

func setup(t *testing.T) *State {
	t.Helper()
	st := newState()
	mustOK(t, st, `{"cmd":"register_device","device":"pump1","reading":100,"date":"2026-01-01"}`)
	mustOK(t, st, `{"cmd":"register_device","device":"pump2","reading":0,"date":"2026-01-10"}`)
	mustOK(t, st, `{"cmd":"add_item","device":"pump1","item":"oil","days":30,"usage":500}`)
	mustOK(t, st, `{"cmd":"add_item","device":"pump1","item":"filter","days":60}`)
	mustOK(t, st, `{"cmd":"add_item","device":"pump2","item":"belt","usage":100}`)
	return st
}

func TestValidationErrors(t *testing.T) {
	st := setup(t)
	cases := []string{
		`{"cmd":"register_device","device":"pump1","reading":0,"date":"2026-01-01"}`, // 重复设备
		`{"cmd":"register_device","device":"x","reading":-1,"date":"2026-01-01"}`,    // 负读数
		`{"cmd":"register_device","device":"x","reading":0,"date":"2026-02-30"}`,     // 不存在的日期
		`{"cmd":"add_item","device":"ghost","item":"a","days":1}`,                    // 未知设备
		`{"cmd":"add_item","device":"pump1","item":"oil","days":1}`,                  // 重复项目
		`{"cmd":"add_item","device":"pump1","item":"new"}`,                           // 无周期
		`{"cmd":"add_item","device":"pump1","item":"new","days":0}`,                  // 非正日间隔
		`{"cmd":"add_item","device":"pump1","item":"new","usage":-5}`,                // 非正运行量
		`{"cmd":"add_reading","device":"pump1","reading":50,"date":"2026-01-05"}`,    // 读数下降
		`{"cmd":"add_reading","device":"pump1","reading":150,"date":"2026-01-01"}`,   // 日期不递增
		`{"cmd":"complete","device":"pump1","item":"ghost"}`,                         // 未知项目
		`{"cmd":"bogus"}`, // 未知命令
	}
	for _, c := range cases {
		mustFail(t, st, c)
	}
	// 状态未被错误命令改变
	if len(st.Devices) != 2 || len(st.Devices["pump1"].Items) != 2 {
		t.Fatal("错误命令改变了状态")
	}
	if got := st.Devices["pump1"].latest().Value; got != 100 {
		t.Fatalf("读数被改变: %v", got)
	}
}

func TestPlanDueAndOrdering(t *testing.T) {
	st := setup(t)
	mustOK(t, st, `{"cmd":"add_reading","device":"pump1","reading":650,"date":"2026-01-20"}`)
	// oil: 运行量 100+500=600 <= 650 到期；日历 2026-01-31 剩 11 天
	// filter: 日历 2026-03-02 剩 41 天
	resp := mustOK(t, st, `{"cmd":"plan","date":"2026-01-20"}`)
	if len(resp.Plan) != 3 {
		t.Fatalf("期望 3 条计划，得到 %d", len(resp.Plan))
	}
	first := resp.Plan[0]
	if !first.Due || first.Device != "pump1" || first.Item != "oil" {
		t.Fatalf("到期项应排第一: %+v", first)
	}
	if *first.RemainingUsage != -50 {
		t.Fatalf("剩余运行量应为 -50，得到 %v", *first.RemainingUsage)
	}
	if *first.RemainingDays != 11 {
		t.Fatalf("剩余天数应为 11，得到 %v", *first.RemainingDays)
	}
	for _, e := range resp.Plan[1:] {
		if e.Due {
			t.Fatalf("非到期项排在到期项后: %+v", e)
		}
	}
}

func TestPlanDateTooEarly(t *testing.T) {
	st := setup(t)
	mustFail(t, st, `{"cmd":"plan","date":"2025-12-31"}`)
	mustOK(t, st, `{"cmd":"plan","date":"2026-01-10"}`)
}

func TestCompleteResetsAndHistory(t *testing.T) {
	st := setup(t)
	mustOK(t, st, `{"cmd":"add_reading","device":"pump1","reading":700,"date":"2026-02-01"}`)
	mustOK(t, st, `{"cmd":"complete","device":"pump1","item":"oil"}`)
	// 完成后基准重置为 (2026-02-01, 700)，不再到期
	resp := mustOK(t, st, `{"cmd":"plan","date":"2026-02-01"}`)
	for _, e := range resp.Plan {
		if e.Item == "oil" {
			if e.Due {
				t.Fatal("完成后 oil 不应到期")
			}
			if *e.NextDate != "2026-03-03" || *e.NextReading != 1200 {
				t.Fatalf("基准未重置: %+v", e)
			}
		}
	}
	h := mustOK(t, st, `{"cmd":"history","device":"pump1","item":"oil"}`)
	if len(h.History) != 1 || h.History[0].Date != "2026-02-01" || h.History[0].Reading != 700 {
		t.Fatalf("历史记录错误: %+v", h.History)
	}
	// 再次完成只影响 oil，filter 历史为空
	mustOK(t, st, `{"cmd":"complete","device":"pump1","item":"oil"}`)
	h = mustOK(t, st, `{"cmd":"history","device":"pump1","item":"filter"}`)
	if len(h.History) != 0 {
		t.Fatal("filter 不应有历史")
	}
	h = mustOK(t, st, `{"cmd":"history"}`)
	if len(h.History) != 2 {
		t.Fatalf("全部历史应为 2 条，得到 %d", len(h.History))
	}
	mustFail(t, st, `{"cmd":"history","device":"ghost"}`)
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/state.json"
	st := setup(t)
	if err := st.save(path); err != nil {
		t.Fatal(err)
	}
	st2, err := loadState(path)
	if err != nil {
		t.Fatal(err)
	}
	resp := mustOK(t, st2, `{"cmd":"plan","date":"2026-01-15"}`)
	if len(resp.Plan) != 3 {
		t.Fatal("重启后状态丢失")
	}
	// 不存在的状态文件返回空状态
	st3, err := loadState(dir + "/none.json")
	if err != nil || len(st3.Devices) != 0 {
		t.Fatal("缺失文件应返回空状态")
	}
}

func TestReadingsMonotonic(t *testing.T) {
	st := setup(t)
	mustOK(t, st, `{"cmd":"add_reading","device":"pump1","reading":100,"date":"2026-01-02"}`) // 允许相等
	mustOK(t, st, `{"cmd":"add_reading","device":"pump1","reading":120.5,"date":"2026-01-03"}`)
	if got := st.Devices["pump1"].latest().Value; got != 120.5 {
		t.Fatalf("最新读数错误: %v", got)
	}
	if !strings.Contains(mustFail(t, st, `{"cmd":"add_reading","device":"pump1","reading":130,"date":"2026-01-03"}`).Error, "严格晚于") {
		t.Fatal("应拒绝非递增日期")
	}
}
