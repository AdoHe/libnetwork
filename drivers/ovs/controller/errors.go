package controller

import (
	"fmt"
)

// ErrBadCIDRFormat is returned when the provided network cidr is invalid.
type ErrBadCIDRFormat string

func (ebcf ErrBadCIDRFormat) Error() string {
	return fmt.Sprintf("")
}

// BadRequest denotes the type of this error
func (ebcd ErrBadCIDRFormat) BadRequest() {}

// ErrInvalidIPAddr is returned when provided gw addr is invalid.
type ErrInvalidGWAddr string

func (eigwa ErrInvalidGWAddr) Error() string {
	return fmt.Sprintf("")
}

// BadRequest denotes the type of this error
func (eigwa ErrInvalidGWAddr) BadRequest() {}

// ErrPostError is returned when a post request failed.
type ErrPostError string

func (epe ErrPostError) Error() string {
	return fmt.Sprintf("failed to do post request: %s", string(epe))
}

// BadRequest denotes the type of this error
func (epe ErrPostError) BadRequest() {}

// ErrResultError is returned when the response
// result is not zero
type ErrResultError string

func (ere ErrResultError) Error() string {
	return fmt.Sprintf("got response result %s", string(ere))
}

// BadRequest denotes the type of this error
func (ere ErrResultError) BadRequest() {}
