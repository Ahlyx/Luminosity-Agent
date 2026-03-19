package memory

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type Store struct {
	Path string
}

type Data struct {
	Facts   []string `json:"facts"`
	Summary string   `json:"summary"`
}

func (s Store) Load() (Data, error) {
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o755); err != nil {
		return Data{}, err
	}

	b, err := os.ReadFile(s.Path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Data{Facts: []string{}}, nil
		}
		return Data{}, err
	}

	if len(b) == 0 {
		return Data{Facts: []string{}}, nil
	}

	var out Data
	if err := json.Unmarshal(b, &out); err != nil {
		return Data{}, fmt.Errorf("corrupted")
	}

	if out.Facts == nil {
		out.Facts = []string{}
	}

	return out, nil
}

func (s Store) Save(data Data) error {
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.Path, b, 0o644)
}

func (s Store) Reset() error {
	return s.Save(Data{Facts: []string{}, Summary: ""})
}
