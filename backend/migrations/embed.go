package migrations

import (
	"embed"
	"io/fs"
	"path"
	"strings"
)

//go:embed sqlite/*.sql postgres/*.sql
var files embed.FS

type File struct {
	Name string
	Path string
}

func Filenames(dialect string) ([]File, error) {
	dialect = strings.ToLower(strings.TrimSpace(dialect))
	entries, err := fs.ReadDir(files, dialect)
	if err != nil {
		return nil, err
	}

	names := make([]File, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		names = append(names, File{
			Name: entry.Name(),
			Path: path.Join(dialect, entry.Name()),
		})
	}

	return names, nil
}

func Read(name string) ([]byte, error) {
	return files.ReadFile(name)
}
