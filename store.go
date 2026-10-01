package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"time"
)

// Reading 是设备在某个日期记录的一次累计运行量读数。
type Reading struct {
	Date  string  `json:"date"`
	Value float64 `json:"value"`
}

// Completion 记录一次保养完成时的基准信息。
type Completion struct {
	Date    string  `json:"date"`
	Reading float64 `json:"reading"`
}

// Item 是一个保养项目，至少配置一种周期。
type Item struct {
	Days        *int         `json:"days,omitempty"`  // 日历周期（天）
	Usage       *float64     `json:"usage,omitempty"` // 运行量周期
	BaseDate    string       `json:"base_date"`
	BaseReading float64      `json:"base_reading"`
	History     []Completion `json:"history,omitempty"`
}

// Device 是一台设备及其读数台账和保养项目。
type Device struct {
	Readings []Reading        `json:"readings"`
	Items    map[string]*Item `json:"items"`
}

// State 是全部持久化状态。
type State struct {
	Devices map[string]*Device `json:"devices"`
}

func newState() *State {
	return &State{Devices: map[string]*Device{}}
}

// loadState 从路径加载状态；文件不存在时返回空状态。
func loadState(path string) (*State, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return newState(), nil
	}
	if err != nil {
		return nil, err
	}
	st := newState()
	if err := json.Unmarshal(data, st); err != nil {
		return nil, fmt.Errorf("状态文件损坏: %w", err)
	}
	if st.Devices == nil {
		st.Devices = map[string]*Device{}
	}
	return st, nil
}

// save 将状态原子化写入磁盘。
func (s *State) save(path string) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// parseDate 严格校验 YYYY-MM-DD 且日期必须真实存在。
func parseDate(s string) (time.Time, error) {
	t, err := time.ParseInLocation("2006-01-02", s, time.UTC)
	if err != nil || t.Format("2006-01-02") != s {
		return time.Time{}, fmt.Errorf("非法日期 %q（需为真实存在的 YYYY-MM-DD）", s)
	}
	return t, nil
}

// checkReading 校验读数为有限非负数。
func checkReading(v float64) error {
	if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
		return fmt.Errorf("读数必须为有限非负数，得到 %v", v)
	}
	return nil
}

// latest 返回设备最新读数。
func (d *Device) latest() Reading {
	return d.Readings[len(d.Readings)-1]
}
