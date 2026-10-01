package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
)

func main() {
	statePath := flag.String("state", "", "状态文件路径")
	flag.Parse()
	if *statePath == "" {
		fmt.Fprintln(os.Stderr, "usage: equipment-maintenance-planner --state <path>")
		os.Exit(2)
	}
	if err := run(*statePath); err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
}

// errorLine 是可定位错误的输出结构。
type errorLine struct {
	OK      bool   `json:"ok"`
	Line    int    `json:"line"`
	Error   string `json:"error"`
	Command string `json:"command,omitempty"`
}

func run(statePath string) error {
	st, err := loadState(statePath)
	if err != nil {
		return fmt.Errorf("load state: %w", err)
	}
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 1024*1024), 16*1024*1024)
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()
	enc := json.NewEncoder(out)

	lineNo := 0
	for in.Scan() {
		lineNo++
		raw := in.Bytes()
		if len(bytesTrimSpace(raw)) == 0 {
			continue
		}
		var c Command
		if err := json.Unmarshal(raw, &c); err != nil {
			enc.Encode(errorLine{OK: false, Line: lineNo, Error: "invalid JSON: " + err.Error()})
			continue
		}
		resp, err := handle(st, c)
		if err != nil {
			enc.Encode(errorLine{OK: false, Line: lineNo, Error: err.Error(), Command: c.Type})
			continue
		}
		if err := saveState(statePath, st); err != nil {
			enc.Encode(errorLine{OK: false, Line: lineNo, Error: "save state: " + err.Error(), Command: c.Type})
			continue
		}
		resp["ok"] = true
		enc.Encode(resp)
	}
	return in.Err()
}

func bytesTrimSpace(b []byte) []byte {
	start, end := 0, len(b)
	for start < end && (b[start] == ' ' || b[start] == '\t' || b[start] == '\r' || b[start] == '\n') {
		start++
	}
	for end > start && (b[end-1] == ' ' || b[end-1] == '\t' || b[end-1] == '\r' || b[end-1] == '\n') {
		end--
	}
	return b[start:end]
}
