package application

import "errors"

var (
	ErrNotFound     = errors.New("not found")
	ErrAlreadyExist = errors.New("already exists")
	ErrInvalidInput = errors.New("invalid input")
)
