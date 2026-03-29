package sound

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Pack struct {
	Name   string            `json:"name"`
	Dir    string            `json:"dir"`
	Sounds map[string]string `json:"sounds"` // event -> filename
}

type Manifest struct {
	Name    string            `json:"name"`
	Version string            `json:"version"`
	Sounds  map[string]string `json:"sounds"`
}

// SoundEvents lists all possible sound event names
var SoundEvents = []string{
	"throw", "single", "double", "triple", "bull", "dbull", "miss",
	"bust", "gameon", "gameshot", "matchshot",
	"180", "hatTrick", "tonPlus", "highTon", "lowTon",
}

// ListPacks discovers all sound packs in the given directory
func ListPacks(soundsDir string) ([]Pack, error) {
	entries, err := os.ReadDir(soundsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Pack{}, nil
		}
		return nil, fmt.Errorf("read sounds dir: %w", err)
	}

	var packs []Pack
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		manifestPath := filepath.Join(soundsDir, entry.Name(), "manifest.json")
		data, err := os.ReadFile(manifestPath)
		if err != nil {
			continue // skip dirs without manifest
		}

		var m Manifest
		if err := json.Unmarshal(data, &m); err != nil {
			continue
		}

		packs = append(packs, Pack{
			Name:   m.Name,
			Dir:    entry.Name(),
			Sounds: m.Sounds,
		})
	}

	return packs, nil
}

// EnsurePack creates a pack directory and empty manifest if they don't exist.
func EnsurePack(soundsDir, packName string) error {
	packDir := filepath.Join(soundsDir, packName)
	if err := os.MkdirAll(packDir, 0755); err != nil {
		return fmt.Errorf("create pack dir: %w", err)
	}
	manifestPath := filepath.Join(packDir, "manifest.json")
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		m := Manifest{Name: packName, Version: "1.0", Sounds: make(map[string]string)}
		data, _ := json.MarshalIndent(m, "", "  ")
		return os.WriteFile(manifestPath, data, 0644)
	}
	return nil
}

// GetPack reads a single pack by directory name.
func GetPack(soundsDir, packName string) (*Pack, error) {
	manifestPath := filepath.Join(soundsDir, packName, "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	if m.Sounds == nil {
		m.Sounds = make(map[string]string)
	}
	return &Pack{Name: m.Name, Dir: packName, Sounds: m.Sounds}, nil
}

// UpdatePackSound sets event → filename in the pack manifest.
func UpdatePackSound(soundsDir, packName, event, filename string) error {
	packDir := filepath.Join(soundsDir, packName)
	manifestPath := filepath.Join(packDir, "manifest.json")

	var m Manifest
	if data, err := os.ReadFile(manifestPath); err == nil {
		json.Unmarshal(data, &m)
	}
	if m.Sounds == nil {
		m.Sounds = make(map[string]string)
	}
	if m.Name == "" {
		m.Name = packName
	}
	if m.Version == "" {
		m.Version = "1.0"
	}
	m.Sounds[event] = filename
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(manifestPath, data, 0644)
}

// RemovePackSound deletes a sound event from the manifest and removes the file.
func RemovePackSound(soundsDir, packName, event string) error {
	packDir := filepath.Join(soundsDir, packName)
	manifestPath := filepath.Join(packDir, "manifest.json")

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	if filename, ok := m.Sounds[event]; ok && filename != "" {
		os.Remove(filepath.Join(packDir, filename)) // best-effort
	}
	delete(m.Sounds, event)
	out, _ := json.MarshalIndent(m, "", "  ")
	return os.WriteFile(manifestPath, out, 0644)
}

// CreateDefaultManifest creates a template manifest.json
func CreateDefaultManifest(dir string) error {
	m := Manifest{
		Name:    "Default",
		Version: "1.0",
		Sounds:  make(map[string]string),
	}
	for _, event := range SoundEvents {
		m.Sounds[event] = event + ".mp3"
	}

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(dir, "manifest.json"), data, 0644)
}
