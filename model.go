package main

import (
	"fmt"
	"time"
)

// DateLayout 是协议规定的日期格式，按自然日解析。
const DateLayout = "2006-01-02"

// parseDate 解析 YYYY-MM-DD，拒绝不存在的日期（如 2021-02-29）。
func parseDate(s string) (time.Time, error) {
	if len(s) != len(DateLayout) {
		return time.Time{}, fmt.Errorf("invalid date %q: expected YYYY-MM-DD", s)
	}
	t, err := time.Parse(DateLayout, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date %q: %v", s, err)
	}
	// time.Parse 会把 2021-02-29 归一化为 3 月日期，往返检查可拦截。
	if t.Format(DateLayout) != s {
		return time.Time{}, fmt.Errorf("invalid date %q: date does not exist", s)
	}
	return t, nil
}

// MaintenanceItem 描述一个保养项目，各项目独立累计。
type MaintenanceItem struct {
	ID        string  `json:"id"`
	Days      int     `json:"days_interval,omitempty"`  // 正整数日间隔，0 表示不启用
	Usage     float64 `json:"usage_interval,omitempty"` // 正运行量间隔，0 表示不启用
	BaseDate  string  `json:"base_date"`                // 最近一次重置基准日期
	BaseUsage float64 `json:"base_usage"`               // 最近一次重置基准读数
	Enabled   bool    `json:"enabled"`
}

// CompletionRecord 是一条保养完成历史。
type CompletionRecord struct {
	Date  string  `json:"date"`
	Usage float64 `json:"usage"`
}

// Equipment 是一台设备的台账。
type Equipment struct {
	ID         string                        `json:"id"`
	LatestDate string                        `json:"latest_date"`
	LatestRead float64                       `json:"latest_reading"`
	Items      map[string]*MaintenanceItem   `json:"items"`
	History    map[string][]CompletionRecord `json:"history"`
}

// State 是持久化到磁盘的完整状态。
type State struct {
	Equipment map[string]*Equipment `json:"equipment"`
}

func newState() *State {
	return &State{Equipment: map[string]*Equipment{}}
}

func (s *State) equipment(id string) (*Equipment, error) {
	e, ok := s.Equipment[id]
	if !ok {
		return nil, fmt.Errorf("unknown equipment %q", id)
	}
	return e, nil
}

func (e *Equipment) item(id string) (*MaintenanceItem, error) {
	m, ok := e.Items[id]
	if !ok {
		return nil, fmt.Errorf("unknown item %q for equipment %q", id, e.ID)
	}
	return m, nil
}
