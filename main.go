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
		fmt.Fprintln(os.Stderr, "用法: go run . --state 状态路径")
		os.Exit(2)
	}
	st, err := loadState(*statePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "加载状态失败:", err)
		os.Exit(1)
	}
	if err := run(st, *statePath, os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// run 逐行读取 JSON 命令并逐行输出 JSON 响应。
func run(st *State, statePath string, in *os.File, out *os.File) error {
	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 0, 1<<20), 1<<20)
	enc := json.NewEncoder(out)
	line := 0
	for sc.Scan() {
		line++
		text := sc.Bytes()
		if len(trimSpace(text)) == 0 {
			continue
		}
		var req Request
		var resp Response
		changed := false
		if err := json.Unmarshal(text, &req); err != nil {
			resp = Response{OK: false, Error: fmt.Sprintf("第 %d 行 JSON 解析失败: %v", line, err)}
		} else {
			resp, changed = dispatch(st, &req)
			if !resp.OK {
				resp.Error = fmt.Sprintf("第 %d 行: %s", line, resp.Error)
			}
		}
		if changed {
			if err := st.save(statePath); err != nil {
				resp = Response{OK: false, Cmd: req.Cmd, Error: fmt.Sprintf("第 %d 行: 状态保存失败: %v", line, err)}
			}
		}
		if err := enc.Encode(resp); err != nil {
			return err
		}
	}
	return sc.Err()
}

func trimSpace(b []byte) []byte {
	start, end := 0, len(b)
	for start < end && (b[start] == ' ' || b[start] == '\t' || b[start] == '\r') {
		start++
	}
	for end > start && (b[end-1] == ' ' || b[end-1] == '\t' || b[end-1] == '\r') {
		end--
	}
	return b[start:end]
}
