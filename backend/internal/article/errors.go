package article

import "errors"

var (
	ErrArticleNotFound = errors.New("article not found")
	ErrDuplicateSlug   = errors.New("article slug already exists")
)
