package migrations

import (
	"embed"
	"io/fs"
)

//go:embed *.sql
var files embed.FS

func Filenames() ([]string, error) {
	entries, err := fs.ReadDir(files, ".")
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		names = append(names, entry.Name())
	}

	return names, nil
}

func Read(name string) ([]byte, error) {
	return files.ReadFile(name)
}
