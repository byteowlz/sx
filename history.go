package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func getStateDir() string {
	stateHome := os.Getenv("XDG_STATE_HOME")
	if stateHome == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		stateHome = filepath.Join(homeDir, ".local", "state")
	}
	return filepath.Join(stateHome, "sx")
}

func getHistoryFile() string {
	return filepath.Join(getStateDir(), "history")
}

func appendHistory(query string) error {
	if !config.HistoryEnabled || query == "" {
		return nil
	}

	stateDir := getStateDir()
	if stateDir == "" {
		return nil
	}

	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return err
	}

	historyFile := getHistoryFile()

	f, err := os.OpenFile(historyFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	entry := fmt.Sprintf("%s\t%s\n", time.Now().Format(time.RFC3339), query)
	_, err = f.WriteString(entry)
	if err != nil {
		return err
	}

	// Trim history if it exceeds max
	return trimHistory()
}

func trimHistory() error {
	maxHistory := config.MaxHistory
	if maxHistory <= 0 {
		maxHistory = defaultMaxHistory
	}

	historyFile := getHistoryFile()
	lines, err := readHistoryLines()
	if err != nil {
		return err
	}

	if len(lines) <= maxHistory {
		return nil
	}

	// Keep only the last maxHistory entries
	lines = lines[len(lines)-maxHistory:]

	f, err := os.Create(historyFile)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, line := range lines {
		fmt.Fprintln(f, line)
	}

	return nil
}

func readHistoryLines() ([]string, error) {
	historyFile := getHistoryFile()

	f, err := os.Open(historyFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			lines = append(lines, line)
		}
	}

	return lines, scanner.Err()
}

type HistoryEntry struct {
	Timestamp time.Time
	Query     string
}

func loadHistory() ([]HistoryEntry, error) {
	lines, err := readHistoryLines()
	if err != nil {
		return nil, err
	}

	var entries []HistoryEntry
	for _, line := range lines {
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			continue
		}

		ts, err := time.Parse(time.RFC3339, parts[0])
		if err != nil {
			continue
		}

		entries = append(entries, HistoryEntry{
			Timestamp: ts,
			Query:     parts[1],
		})
	}

	return entries, nil
}

func printHistory(limit int) error {
	entries, err := loadHistory()
	if err != nil {
		return fmt.Errorf("failed to load history: %v", err)
	}

	if len(entries) == 0 {
		fmt.Println("No search history.")
		return nil
	}

	// Show most recent first
	start := 0
	if limit > 0 && limit < len(entries) {
		start = len(entries) - limit
	}

	for _, entry := range entries[start:] {
		fmt.Printf("  %s  %s\n", entry.Timestamp.Format("2006-01-02 15:04"), entry.Query)
	}

	return nil
}

func clearHistory() error {
	historyFile := getHistoryFile()
	if err := os.Remove(historyFile); err != nil && !os.IsNotExist(err) {
		return err
	}
	fmt.Println("History cleared.")
	return nil
}
