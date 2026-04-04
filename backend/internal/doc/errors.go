package doc

import "errors"

var (
	ErrCategoryNotFound = errors.New("doc category not found")
	ErrPageNotFound     = errors.New("doc page not found")
	ErrDuplicateSlug    = errors.New("doc slug already exists")
)
