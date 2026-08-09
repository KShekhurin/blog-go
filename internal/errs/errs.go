package errs

import (
	"errors"
	"fmt"
)

var (
	ErrNotFound       = errors.New("resource does not exist")
	ErrAlreadyDeleted = errors.New("resource is already deleted")
	ErrAlreadyExists  = errors.New("resource already exists")
	ErrUnauthorized   = errors.New("unauthorized")
	ErrInvalidInput   = errors.New("invalid input")
)

type NotFoundError struct {
	Resource string
	ID       string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("%s with id %s not found", e.Resource, e.ID)
}

func (e *NotFoundError) Is(target error) bool {
	return target == ErrNotFound
}
