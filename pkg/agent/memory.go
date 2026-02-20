// PicoClaw - Ultra-lightweight personal AI agent
// Inspired by and based on nanobot: https://github.com/HKUDS/nanobot
// License: MIT
//
// Copyright (c) 2026 PicoClaw contributors

package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// MemoryStore manages persistent memory for the agent.
// - Long-term memory: memory/profile.json
// - Daily notes: memory/YYYYMM/YYYYMMDD.md
type MemoryStore struct {
	mu          sync.RWMutex
	workspace   string
	memoryDir   string
	profileFile string
}

// NewMemoryStore creates a new MemoryStore with the given workspace path.
// It ensures the memory directory exists.
func NewMemoryStore(workspace string) *MemoryStore {
	memoryDir := filepath.Join(workspace, "memory")
	profileFile := filepath.Join(memoryDir, "profile.json")

	// Ensure memory directory exists
	os.MkdirAll(memoryDir, 0755)

	return &MemoryStore{
		workspace:   workspace,
		memoryDir:   memoryDir,
		profileFile: profileFile,
	}
}

// getTodayFile returns the path to today's daily note file (memory/YYYYMM/YYYYMMDD.md).
func (ms *MemoryStore) getTodayFile() string {
	today := time.Now().Format("20060102") // YYYYMMDD
	monthDir := today[:6]                  // YYYYMM
	filePath := filepath.Join(ms.memoryDir, monthDir, today+".md")
	return filePath
}

// ReadProfile reads the long-term profile JSON safely.
func (ms *MemoryStore) ReadProfile() map[string]string {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	profile := make(map[string]string)
	data, err := os.ReadFile(ms.profileFile)
	if err == nil && len(data) > 0 {
		_ = json.Unmarshal(data, &profile)
	}
	return profile
}

// WriteProfileKey safely updates or adds a key in the profile.
func (ms *MemoryStore) WriteProfileKey(key, value string) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	profile := make(map[string]string)
	data, err := os.ReadFile(ms.profileFile)
	if err == nil && len(data) > 0 {
		if err := json.Unmarshal(data, &profile); err != nil {
			return fmt.Errorf("failed to parse profile.json (file might be corrupted): %w", err)
		}
	}

	profile[key] = value

	newData, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal profile data: %w", err)
	}
	return os.WriteFile(ms.profileFile, newData, 0644)
}

// DeleteProfileKey safely removes a key from the profile.
func (ms *MemoryStore) DeleteProfileKey(key string) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	profile := make(map[string]string)
	data, err := os.ReadFile(ms.profileFile)
	if err == nil && len(data) > 0 {
		if err := json.Unmarshal(data, &profile); err != nil {
			return fmt.Errorf("failed to parse profile.json (file might be corrupted): %w", err)
		}
	}

	if _, exists := profile[key]; !exists {
		return nil // Key doesn't exist, nothing to do
	}

	delete(profile, key)

	newData, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal profile data: %w", err)
	}
	return os.WriteFile(ms.profileFile, newData, 0644)
}

// ReadToday reads today's daily note.
// Returns empty string if the file doesn't exist.
func (ms *MemoryStore) ReadToday() string {
	todayFile := ms.getTodayFile()
	if data, err := os.ReadFile(todayFile); err == nil {
		return string(data)
	}
	return ""
}

// AppendToday appends content to today's daily note.
// If the file doesn't exist, it creates a new file with a date header.
func (ms *MemoryStore) AppendToday(content string) error {
	todayFile := ms.getTodayFile()

	// Ensure month directory exists
	monthDir := filepath.Dir(todayFile)
	os.MkdirAll(monthDir, 0755)

	var existingContent string
	if data, err := os.ReadFile(todayFile); err == nil {
		existingContent = string(data)
	}

	var newContent string
	if existingContent == "" {
		// Add header for new day
		header := fmt.Sprintf("# %s\n\n", time.Now().Format("2006-01-02"))
		newContent = header + content
	} else {
		// Append to existing content
		newContent = existingContent + "\n" + content
	}

	return os.WriteFile(todayFile, []byte(newContent), 0644)
}

// GetRecentDailyNotes returns daily notes from the last N days.
// Contents are joined with "---" separator.
func (ms *MemoryStore) GetRecentDailyNotes(days int) string {
	var notes []string

	for i := 0; i < days; i++ {
		date := time.Now().AddDate(0, 0, -i)
		dateStr := date.Format("20060102") // YYYYMMDD
		monthDir := dateStr[:6]            // YYYYMM
		filePath := filepath.Join(ms.memoryDir, monthDir, dateStr+".md")

		if data, err := os.ReadFile(filePath); err == nil {
			notes = append(notes, string(data))
		}
	}

	if len(notes) == 0 {
		return ""
	}

	// Join with separator
	var result string
	for i, note := range notes {
		if i > 0 {
			result += "\n\n---\n\n"
		}
		result += note
	}
	return result
}

// GetMemoryContext returns formatted memory context for the agent prompt.
func (ms *MemoryStore) GetMemoryContext() string {
	var parts []string

	// Long-term memory (Profile)
	profile := ms.ReadProfile()
	if len(profile) > 0 {
		var profileStr string
		for k, v := range profile {
			profileStr += fmt.Sprintf("- **%s**: %s\n", k, v)
		}
		parts = append(parts, "## Core Profile (Facts & Preferences)\n\n"+profileStr)
	}

	// Recent daily notes (last 3 days)
	recentNotes := ms.GetRecentDailyNotes(3)
	if recentNotes != "" {
		parts = append(parts, "## Recent Daily Notes\n\n"+recentNotes)
	}

	if len(parts) == 0 {
		return ""
	}

	var result string
	for i, part := range parts {
		if i > 0 {
			result += "\n\n---\n\n"
		}
		result += part
	}
	return fmt.Sprintf("# Memory\n\n%s", result)
}

// MigrateLegacyUserMD checks for a legacy USER.md file and migrates it to the JSON profile.
func (ms *MemoryStore) MigrateLegacyUserMD() error {
	userMDPath := filepath.Join(ms.workspace, "USER.md")
	data, err := os.ReadFile(userMDPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Nothing to migrate
		}
		return err
	}

	err = ms.WriteProfileKey("legacy_user_preferences", string(data))
	if err != nil {
		return err
	}

	bakPath := filepath.Join(ms.workspace, "USER.md.bak")
	return os.Rename(userMDPath, bakPath)
}
