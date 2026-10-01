package main

import (
	"fmt"
)

// Request 是一行 JSON 命令。
type Request struct {
	Cmd     string   `json:"cmd"`
	Device  string   `json:"device"`
	Item    string   `json:"item"`
	Date    string   `json:"date"`
	Reading *float64 `json:"reading"`
	Days    *int     `json:"days"`
	Usage   *float64 `json:"usage"`
}

// Response 是一行 JSON 响应。
type Response struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
	Cmd   string `json:"cmd,omitempty"`
	// 查询类结果
	Plan    []PlanEntry    `json:"plan,omitempty"`
	History []HistoryEntry `json:"history,omitempty"`
}

func fail(cmd string, err error) Response {
	return Response{OK: false, Cmd: cmd, Error: err.Error()}
}

// dispatch 校验并执行一条命令；出错时保证状态未被修改。
func dispatch(st *State, req *Request) (resp Response, changed bool) {
	switch req.Cmd {
	case "register_device":
		return cmdRegisterDevice(st, req)
	case "add_item":
		return cmdAddItem(st, req)
	case "add_reading":
		return cmdAddReading(st, req)
	case "complete":
		return cmdComplete(st, req)
	case "plan":
		return cmdPlan(st, req), false
	case "history":
		return cmdHistory(st, req), false
	default:
		return fail(req.Cmd, fmt.Errorf("未知命令 %q", req.Cmd)), false
	}
}

func cmdRegisterDevice(st *State, req *Request) (Response, bool) {
	if req.Device == "" {
		return fail(req.Cmd, fmt.Errorf("缺少设备标识 device")), false
	}
	if _, dup := st.Devices[req.Device]; dup {
		return fail(req.Cmd, fmt.Errorf("设备 %q 已存在", req.Device)), false
	}
	if req.Reading == nil {
		return fail(req.Cmd, fmt.Errorf("缺少初始读数 reading")), false
	}
	if err := checkReading(*req.Reading); err != nil {
		return fail(req.Cmd, err), false
	}
	if _, err := parseDate(req.Date); err != nil {
		return fail(req.Cmd, err), false
	}
	st.Devices[req.Device] = &Device{
		Readings: []Reading{{Date: req.Date, Value: *req.Reading}},
		Items:    map[string]*Item{},
	}
	return Response{OK: true, Cmd: req.Cmd}, true
}

func cmdAddItem(st *State, req *Request) (Response, bool) {
	dev, resp, ok := lookupDevice(st, req)
	if !ok {
		return resp, false
	}
	if req.Item == "" {
		return fail(req.Cmd, fmt.Errorf("缺少项目标识 item")), false
	}
	if _, dup := dev.Items[req.Item]; dup {
		return fail(req.Cmd, fmt.Errorf("设备 %q 的项目 %q 已存在", req.Device, req.Item)), false
	}
	if req.Days == nil && req.Usage == nil {
		return fail(req.Cmd, fmt.Errorf("项目至少需配置 days 或 usage 一种周期")), false
	}
	if req.Days != nil && *req.Days <= 0 {
		return fail(req.Cmd, fmt.Errorf("days 必须为正整数，得到 %d", *req.Days)), false
	}
	if req.Usage != nil && (*req.Usage <= 0 || checkReading(*req.Usage) != nil) {
		return fail(req.Cmd, fmt.Errorf("usage 必须为有限正数，得到 %v", *req.Usage)), false
	}
	latest := dev.latest()
	dev.Items[req.Item] = &Item{
		Days:        req.Days,
		Usage:       req.Usage,
		BaseDate:    latest.Date,
		BaseReading: latest.Value,
	}
	return Response{OK: true, Cmd: req.Cmd}, true
}

func cmdAddReading(st *State, req *Request) (Response, bool) {
	dev, resp, ok := lookupDevice(st, req)
	if !ok {
		return resp, false
	}
	if req.Reading == nil {
		return fail(req.Cmd, fmt.Errorf("缺少读数 reading")), false
	}
	if err := checkReading(*req.Reading); err != nil {
		return fail(req.Cmd, err), false
	}
	if _, err := parseDate(req.Date); err != nil {
		return fail(req.Cmd, err), false
	}
	latest := dev.latest()
	if req.Date <= latest.Date {
		return fail(req.Cmd, fmt.Errorf("日期 %s 必须严格晚于最新读数日期 %s", req.Date, latest.Date)), false
	}
	if *req.Reading < latest.Value {
		return fail(req.Cmd, fmt.Errorf("读数 %v 不得小于最新读数 %v", *req.Reading, latest.Value)), false
	}
	dev.Readings = append(dev.Readings, Reading{Date: req.Date, Value: *req.Reading})
	return Response{OK: true, Cmd: req.Cmd}, true
}

func cmdComplete(st *State, req *Request) (Response, bool) {
	dev, resp, ok := lookupDevice(st, req)
	if !ok {
		return resp, false
	}
	item, resp, ok := lookupItem(dev, req)
	if !ok {
		return resp, false
	}
	latest := dev.latest()
	item.BaseDate = latest.Date
	item.BaseReading = latest.Value
	item.History = append(item.History, Completion{Date: latest.Date, Reading: latest.Value})
	return Response{OK: true, Cmd: req.Cmd}, true
}

func lookupDevice(st *State, req *Request) (*Device, Response, bool) {
	dev, ok := st.Devices[req.Device]
	if !ok {
		return nil, fail(req.Cmd, fmt.Errorf("未知设备 %q", req.Device)), false
	}
	return dev, Response{}, true
}

func lookupItem(dev *Device, req *Request) (*Item, Response, bool) {
	item, ok := dev.Items[req.Item]
	if !ok {
		return nil, fail(req.Cmd, fmt.Errorf("设备 %q 的未知项目 %q", req.Device, req.Item)), false
	}
	return item, Response{}, true
}
