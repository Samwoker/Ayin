package ingest

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// FileLogAdapter searches and parses log entries from local files or directories.
type FileLogAdapter struct {
	baseDir string
}

func NewFileLogAdapter(baseDir string) *FileLogAdapter {
	return &FileLogAdapter{
		baseDir: baseDir,
	}
}

func (a *FileLogAdapter) Name() string {
	return "file"
}

func (a *FileLogAdapter) QueryLogs(ctx context.Context, query LogQuery) ([]LogEntry, error) {
	limit := query.Limit
	if limit <= 0 {
		limit = 100
	}

	var patternRegex *regexp.Regexp
	if query.Pattern != "" {
		if re, err := regexp.Compile("(?i)" + query.Pattern); err == nil {
			patternRegex = re
		}
	}

	var entries []LogEntry

	// Locate candidate log files
	files, err := a.findLogFiles(query.Service)
	if err != nil {
		return nil, err
	}

	for _, path := range files {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		fileEntries, err := a.scanFile(ctx, path, query, patternRegex, limit-len(entries))
		if err != nil {
			continue // Skip unreadable file and proceed with others
		}

		entries = append(entries, fileEntries...)
		if len(entries) >= limit {
			break
		}
	}

	return entries, nil
}

func (a *FileLogAdapter) findLogFiles(service string) ([]string, error) {
	if a.baseDir == "" {
		return nil, nil
	}

	fi, err := os.Stat(a.baseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to access base log path %s: %w", a.baseDir, err)
	}

	// If baseDir is a direct file path
	if !fi.IsDir() {
		return []string{a.baseDir}, nil
	}

	var matched []string
	_ = filepath.Walk(a.baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		name := strings.ToLower(info.Name())
		if strings.HasSuffix(name, ".log") || strings.HasSuffix(name, ".txt") || strings.HasSuffix(name, ".json") {
			if service == "" || strings.Contains(name, strings.ToLower(service)) {
				matched = append(matched, path)
			}
		}
		return nil
	})

	return matched, nil
}

func (a *FileLogAdapter) scanFile(
	ctx context.Context,
	filePath string,
	query LogQuery,
	patternRegex *regexp.Regexp,
	remainingLimit int,
) ([]LogEntry, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var fileEntries []LogEntry

	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Pattern filter check
		if patternRegex != nil {
			if !patternRegex.MatchString(line) {
				continue
			}
		} else if query.Pattern != "" {
			if !strings.Contains(strings.ToLower(line), strings.ToLower(query.Pattern)) {
				continue
			}
		}

		entry := a.parseLine(line, filePath, query.Service)

		// Time range filter
		if !query.StartTime.IsZero() && entry.Timestamp.Before(query.StartTime) {
			continue
		}
		if !query.EndTime.IsZero() && entry.Timestamp.After(query.EndTime) {
			continue
		}

		fileEntries = append(fileEntries, entry)
		if len(fileEntries) >= remainingLimit {
			break
		}
	}

	return fileEntries, scanner.Err()
}

func (a *FileLogAdapter) parseLine(line string, filePath string, defaultService string) LogEntry {
	// Try parsing as structured JSON log
	var jsonMap map[string]interface{}
	if strings.HasPrefix(line, "{") && strings.HasSuffix(line, "}") {
		if err := json.Unmarshal([]byte(line), &jsonMap); err == nil {
			entry := LogEntry{
				Timestamp: time.Now().UTC(),
				Service:   defaultService,
				Level:     "info",
				Labels:    make(map[string]string),
			}

			// Extract service
			for _, k := range []string{"service", "app", "job"} {
				if s, ok := jsonMap[k].(string); ok && s != "" {
					entry.Service = s
					break
				}
			}

			// Extract level
			for _, k := range []string{"level", "severity", "lvl"} {
				if l, ok := jsonMap[k].(string); ok && l != "" {
					entry.Level = strings.ToLower(l)
					break
				}
			}

			// Extract message
			for _, k := range []string{"msg", "message", "text", "error"} {
				if m, ok := jsonMap[k].(string); ok && m != "" {
					entry.Message = m
					break
				}
			}
			if entry.Message == "" {
				entry.Message = line
			}

			// Extract timestamp
			for _, k := range []string{"time", "timestamp", "@timestamp", "ts"} {
				if tStr, ok := jsonMap[k].(string); ok {
					if t, err := time.Parse(time.RFC3339, tStr); err == nil {
						entry.Timestamp = t
						break
					}
				}
			}

			return entry
		}
	}

	// Plain text log fallback
	serviceName := defaultService
	if serviceName == "" {
		serviceName = strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
	}

	return LogEntry{
		Timestamp: time.Now().UTC(),
		Service:   serviceName,
		Level:     inferLevel(line),
		Message:   line,
		Labels:    map[string]string{"file": filepath.Base(filePath)},
	}
}

func inferLevel(line string) string {
	lower := strings.ToLower(line)
	if strings.Contains(lower, "error") || strings.Contains(lower, "fatal") || strings.Contains(lower, "panic") {
		return "error"
	}
	if strings.Contains(lower, "warn") {
		return "warn"
	}
	if strings.Contains(lower, "debug") {
		return "debug"
	}
	return "info"
}
