package repositories

import "errors"

var (
	ErrorDoesNotExist    = errors.New("does not exists")
	ErrorUniqueViolation = errors.New("unique violation")
)
