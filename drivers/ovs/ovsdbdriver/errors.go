package ovsdbdriver

import (
	"fmt"
)

// ErrInvalidOvsDBConnection err is returned when faied to connect to OVSDB.
type ErrInvalidOvsDBConnection struct{}

func (eiodc *ErrInvalidOvsDBConnection) Error() string {
	return "failed to connect to OVSDB"
}

// ErrReplyDisMatchOps err is returned when the number of replies mismatch the ops.
type ErrReplyDisMatchOps struct{}

func (erdmo *ErrReplyDisMatchOps) Error() string {
	return "Number of replies doesn't match operations number"
}

// ErrTransactionError err is returned when an OVSDB Transaction failed to execute.
type ErrTransactionError struct {
	errMsg string
}

func (ete *ErrTransactionError) Error() string {
	return fmt.Sprintf("Transaction Failed due to error %s", ete.errMsg)
}

type ErrBridgeAlreadyExists string

func (ebae ErrBridgeAlreadyExists) Error() string {
	return fmt.Sprintf("Bridge %s already exists, should not create again", string(ebae))
}

// BadRequest denotes the type of error
func (ebae ErrBridgeAlreadyExists) BadRequest() {}

type ErrBridgeNotExists string

func (ebne ErrBridgeNotExists) Error() string {
	return fmt.Sprintf("Bridge %s not exists, can not remove not exist bridge", string(ebne))
}

// BadRequest denotes the type of error
func (ebne ErrBridgeNotExists) BadRequest() {}
