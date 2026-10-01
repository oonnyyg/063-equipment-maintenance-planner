package main

import (
	"fmt"
	"sort"
)

// PlanEntry 是某设备某项目在查询日期的到期状态。
type PlanEntry struct {
	Device string `json:"device"`
	Item   string `json:"item"`
	Due    bool   `json:"due"`
	// 日历周期（启用时填充）
	NextDate      *string `json:"next_date,omitempty"`
	RemainingDays *int    `json:"remaining_days,omitempty"`
	// 运行量周期（启用时填充）
	NextReading    *float64 `json:"next_reading,omitempty"`
	RemainingUsage *float64 `json:"remaining_usage,omitempty"`
}

// HistoryEntry 是一条保养完成历史。
type HistoryEntry struct {
	Device  string  `json:"device"`
	Item    string  `json:"item"`
	Date    string  `json:"date"`
	Reading float64 `json:"reading"`
}

// cmdPlan 计算查询日期的到期清单。使用各设备最新读数，不预测运行量。
func cmdPlan(st *State, req *Request) Response {
	qdate, err := parseDate(req.Date)
	if err != nil {
		return fail(req.Cmd, err)
	}
	// 查询日期不得早于台账内任何读数日。
	for id, dev := range st.Devices {
		for _, r := range dev.Readings {
			rt, _ := parseDate(r.Date)
			if qdate.Before(rt) {
				return fail(req.Cmd, fmt.Errorf("查询日期 %s 早于设备 %q 的读数日 %s", req.Date, id, r.Date))
			}
		}
	}
	var entries []PlanEntry
	for devID, dev := range st.Devices {
		latest := dev.latest()
		for itemID, item := range dev.Items {
			e := PlanEntry{Device: devID, Item: itemID}
			if item.Days != nil {
				base, _ := parseDate(item.BaseDate)
				next := base.AddDate(0, 0, *item.Days)
				nextStr := next.Format("2006-01-02")
				rem := int(next.Sub(qdate).Hours() / 24)
				e.NextDate = &nextStr
				e.RemainingDays = &rem
				if rem <= 0 {
					e.Due = true
				}
			}
			if item.Usage != nil {
				threshold := item.BaseReading + *item.Usage
				rem := threshold - latest.Value
				e.NextReading = &threshold
				e.RemainingUsage = &rem
				if rem <= 0 {
					e.Due = true
				}
			}
			entries = append(entries, e)
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Due != entries[j].Due {
			return entries[i].Due
		}
		if entries[i].Device != entries[j].Device {
			return entries[i].Device < entries[j].Device
		}
		return entries[i].Item < entries[j].Item
	})
	if entries == nil {
		entries = []PlanEntry{}
	}
	return Response{OK: true, Cmd: req.Cmd, Plan: entries}
}

// cmdHistory 返回保养完成历史；device/item 可省略表示不过滤。
func cmdHistory(st *State, req *Request) Response {
	if req.Device != "" {
		dev, ok := st.Devices[req.Device]
		if !ok {
			return fail(req.Cmd, fmt.Errorf("未知设备 %q", req.Device))
		}
		if req.Item != "" {
			if _, ok := dev.Items[req.Item]; !ok {
				return fail(req.Cmd, fmt.Errorf("设备 %q 的未知项目 %q", req.Device, req.Item))
			}
		}
	}
	var out []HistoryEntry
	for devID, dev := range st.Devices {
		if req.Device != "" && req.Device != devID {
			continue
		}
		for itemID, item := range dev.Items {
			if req.Item != "" && req.Item != itemID {
				continue
			}
			for _, h := range item.History {
				out = append(out, HistoryEntry{Device: devID, Item: itemID, Date: h.Date, Reading: h.Reading})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Date != out[j].Date {
			return out[i].Date < out[j].Date
		}
		if out[i].Device != out[j].Device {
			return out[i].Device < out[j].Device
		}
		return out[i].Item < out[j].Item
	})
	if out == nil {
		out = []HistoryEntry{}
	}
	return Response{OK: true, Cmd: req.Cmd, History: out}
}
