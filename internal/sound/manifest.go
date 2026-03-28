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
