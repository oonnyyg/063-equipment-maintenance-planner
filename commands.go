package main

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// Command 是标准输入一行的 JSON 命令。
type Command struct {
	Type           string   `json:"type"`
	EquipmentID    *string  `json:"equipment_id"`
	ItemID         *string  `json:"item_id"`
	ID             *string  `json:"id"`
	InitialReading *float64 `json:"initial_reading"`
	InitialDate    *string  `json:"initial_date"`
	Date           *string  `json:"date"`
	Reading        *float64 `json:"reading"`
	DaysInterval   *int     `json:"days_interval"`
	UsageInterval  *float64 `json:"usage_interval"`
}

func (c Command) str(p *string, name string) (string, error) {
	if p == nil || *p == "" {
		return "", fmt.Errorf("missing %s", name)
	}
	return *p, nil
}

// finiteNonNegative 校验读数为有限、非负数（拒绝 NaN/Inf/负数）。
func finiteNonNegative(v float64) error {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return fmt.Errorf("reading must be finite")
	}
	if v < 0 {
		return fmt.Errorf("reading must be non-negative")
	}
	return nil
}

// Response 是逐行输出的统一 JSON 结构。
type Response map[string]any

// handle 在状态上执行一条命令；返回的错误带定位信息，且任何错误都不改变状态。
func handle(st *State, c Command) (Response, error) {
	switch c.Type {
	case "register_equipment":
		return registerEquipment(st, c)
	case "create_item":
		return createItem(st, c)
	case "add_reading":
		return addReading(st, c)
	case "complete_maintenance":
		return completeMaintenance(st, c)
	case "query_plan":
		return queryPlan(st, c)
	case "maintenance_history":
		return maintenanceHistory(st, c)
	case "selftest":
		return selftest()
	default:
		return nil, fmt.Errorf("unknown command type %q", c.Type)
	}
}

func registerEquipment(st *State, c Command) (Response, error) {
	id, err := c.str(c.EquipmentID, "equipment_id")
	if err != nil {
		return nil, err
	}
	if c.ID != nil && *c.ID != "" {
		id = *c.ID
	}
	if c.InitialReading == nil {
		return nil, fmt.Errorf("missing initial_reading")
	}
	if err := finiteNonNegative(*c.InitialReading); err != nil {
		return nil, err
	}
	if c.InitialDate == nil {
		return nil, fmt.Errorf("missing initial_date")
	}
	d, err := parseDate(*c.InitialDate)
	if err != nil {
		return nil, err
	}
	if _, exists := st.Equipment[id]; exists {
		return nil, fmt.Errorf("duplicate equipment id %q", id)
	}
	st.Equipment[id] = &Equipment{
		ID:         id,
		LatestDate: d.Format(DateLayout),
		LatestRead: *c.InitialReading,
		Items:      map[string]*MaintenanceItem{},
		History:    map[string][]CompletionRecord{},
	}
	return Response{"ok": true, "equipment_id": id}, nil
}

func createItem(st *State, c Command) (Response, error) {
	eqID, err := c.str(c.EquipmentID, "equipment_id")
	if err != nil {
		return nil, err
	}
	itemID, err := c.str(c.ItemID, "item_id")
	if err != nil {
		return nil, err
	}
	e, err := st.equipment(eqID)
	if err != nil {
		return nil, err
	}
	if _, exists := e.Items[itemID]; exists {
		return nil, fmt.Errorf("duplicate item id %q for equipment %q", itemID, eqID)
	}
	days, usage := 0, 0.0
	if c.DaysInterval != nil {
		if *c.DaysInterval <= 0 {
			return nil, fmt.Errorf("days_interval must be a positive integer")
		}
		days = *c.DaysInterval
	}
	if c.UsageInterval != nil {
		if math.IsNaN(*c.UsageInterval) || math.IsInf(*c.UsageInterval, 0) || *c.UsageInterval <= 0 {
			return nil, fmt.Errorf("usage_interval must be a positive finite number")
		}
		usage = *c.UsageInterval
	}
	if days == 0 && usage == 0 {
		return nil, fmt.Errorf("item must configure at least one interval (days_interval or usage_interval)")
	}
	e.Items[itemID] = &MaintenanceItem{
		ID:        itemID,
		Days:      days,
		Usage:     usage,
		BaseDate:  e.LatestDate,
		BaseUsage: e.LatestRead,
		Enabled:   true,
	}
	e.History[itemID] = []CompletionRecord{}
	return Response{"ok": true, "equipment_id": eqID, "item_id": itemID}, nil
}

func addReading(st *State, c Command) (Response, error) {
	eqID, err := c.str(c.EquipmentID, "equipment_id")
	if err != nil {
		return nil, err
	}
	if c.Date == nil {
		return nil, fmt.Errorf("missing date")
	}
	if c.Reading == nil {
		return nil, fmt.Errorf("missing reading")
	}
	if err := finiteNonNegative(*c.Reading); err != nil {
		return nil, err
	}
	d, err := parseDate(*c.Date)
	if err != nil {
		return nil, err
	}
	e, err := st.equipment(eqID)
	if err != nil {
		return nil, err
	}
	last, _ := parseDate(e.LatestDate)
	if !d.After(last) {
		return nil, fmt.Errorf("date %s must be strictly after latest reading date %s",
			*c.Date, e.LatestDate)
	}
	if *c.Reading < e.LatestRead {
		return nil, fmt.Errorf("reading %v must not be lower than latest reading %v",
			*c.Reading, e.LatestRead)
	}
	e.LatestDate = d.Format(DateLayout)
	e.LatestRead = *c.Reading
	return Response{
		"ok":             true,
		"equipment_id":   eqID,
		"latest_date":    e.LatestDate,
		"latest_reading": e.LatestRead,
	}, nil
}

func completeMaintenance(st *State, c Command) (Response, error) {
	eqID, err := c.str(c.EquipmentID, "equipment_id")
	if err != nil {
		return nil, err
	}
	itemID, err := c.str(c.ItemID, "item_id")
	if err != nil {
		return nil, err
	}
	e, err := st.equipment(eqID)
	if err != nil {
		return nil, err
	}
	m, err := e.item(itemID)
	if err != nil {
		return nil, err
	}
	// 仅重置指定项目，以设备最新读数和日期作为新基准。
	m.BaseDate = e.LatestDate
	m.BaseUsage = e.LatestRead
	e.History[itemID] = append(e.History[itemID], CompletionRecord{
		Date:  e.LatestDate,
		Usage: e.LatestRead,
	})
	return Response{
		"ok":           true,
		"equipment_id": eqID,
		"item_id":      itemID,
		"base_date":    m.BaseDate,
		"base_usage":   m.BaseUsage,
	}, nil
}

// PlanItem 是一条计划查询结果。
type PlanItem struct {
	EquipmentID        string  `json:"equipment_id"`
	ItemID             string  `json:"item_id"`
	Due                bool    `json:"due"`
	DaysDue            bool    `json:"days_due"`
	UsageDue           bool    `json:"usage_due"`
	NextDaysDate       string  `json:"next_days_date,omitempty"`
	DaysRemaining      int     `json:"days_remaining,omitempty"`
	NextUsageThreshold float64 `json:"next_usage_threshold,omitempty"`
	UsageRemaining     float64 `json:"usage_remaining,omitempty"`
}

func queryPlan(st *State, c Command) (Response, error) {
	if c.Date == nil {
		return nil, fmt.Errorf("missing date")
	}
	queryDate, err := parseDate(*c.Date)
	if err != nil {
		return nil, err
	}
	// 查询日期不得早于台账内任何读数日。
	for id, e := range st.Equipment {
		last, _ := parseDate(e.LatestDate)
		if queryDate.Before(last) {
			return nil, fmt.Errorf("query date %s is before latest reading date %s of equipment %q",
				*c.Date, e.LatestDate, id)
		}
	}
	items := []PlanItem{}
	for _, e := range st.Equipment {
		for _, m := range e.Items {
			if !m.Enabled {
				continue
			}
			pi := PlanItem{EquipmentID: e.ID, ItemID: m.ID}
			base, _ := parseDate(m.BaseDate)
			if m.Days > 0 {
				next := base.AddDate(0, 0, m.Days)
				pi.NextDaysDate = next.Format(DateLayout)
				pi.DaysRemaining = int(next.Sub(truncateDay(queryDate)).Hours() / 24)
				if pi.DaysRemaining <= 0 {
					pi.DaysDue = true
				}
			}
			if m.Usage > 0 {
				pi.NextUsageThreshold = m.BaseUsage + m.Usage
				// 使用设备最新读数，不预测运行量。
				pi.UsageRemaining = pi.NextUsageThreshold - e.LatestRead
				if pi.UsageRemaining <= 0 {
					pi.UsageDue = true
				}
			}
			pi.Due = pi.DaysDue || pi.UsageDue
			items = append(items, pi)
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Due != items[j].Due {
			return items[i].Due // 到期项优先
		}
		if items[i].EquipmentID != items[j].EquipmentID {
			return items[i].EquipmentID < items[j].EquipmentID
		}
		return items[i].ItemID < items[j].ItemID
	})
	return Response{"ok": true, "date": *c.Date, "items": items}, nil
}

func truncateDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

func maintenanceHistory(st *State, c Command) (Response, error) {
	eqID, err := c.str(c.EquipmentID, "equipment_id")
	if err != nil {
		return nil, err
	}
	e, err := st.equipment(eqID)
	if err != nil {
		return nil, err
	}
	result := map[string][]CompletionRecord{}
	if c.ItemID != nil && *c.ItemID != "" {
		if _, err := e.item(*c.ItemID); err != nil {
			return nil, err
		}
		result[*c.ItemID] = e.History[*c.ItemID]
	} else {
		for id, recs := range e.History {
			result[id] = recs
		}
	}
	return Response{"ok": true, "equipment_id": eqID, "history": result}, nil
}
