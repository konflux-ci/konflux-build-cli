package main

import "github.com/pkg/errors"

func NewError() error {
	return errors.New("fixture error")
}
