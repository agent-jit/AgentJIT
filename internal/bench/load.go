package bench

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// LoadTasks reads tasks from a JSONL file (one Task object per line). Blank
// lines and lines beginning with '#' are ignored so fixtures can carry comments.
func LoadTasks(path string) ([]Task, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var tasks []Task
	sc := bufio.NewScanner(f)
	// Task prompts can be long; raise the line cap well above the default 64KB.
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	line := 0
	for sc.Scan() {
		line++
		text := strings.TrimSpace(sc.Text())
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		var t Task
		if err := json.Unmarshal([]byte(text), &t); err != nil {
			return nil, fmt.Errorf("%s:%d: %w", path, line, err)
		}
		if t.ID == "" {
			return nil, fmt.Errorf("%s:%d: task is missing \"id\"", path, line)
		}
		tasks = append(tasks, t)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return tasks, nil
}
