package domain

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"net"
)

// txAbortClassifier is implemented by a repository whose driver knows which of its errors end
// the transaction (the domain does not import the driver).
type txAbortClassifier interface {
	IsTxAborting(err error) bool
}

// IsTxAborting reports whether err means the transaction repo belongs to is lost or cannot go
// on: a deadlock or lock wait timeout, a cancelled or timed-out context, a broken connection.
// Such an error must reach the caller of InTx. Swallowing it would let COMMIT run on a
// transaction that is already rolled back and report a change that was undone. An error that
// only failed one statement (a constraint, a bad value) leaves the transaction alive.
func IsTxAborting(repo Repository, err error) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded),
		errors.Is(err, driver.ErrBadConn), errors.Is(err, sql.ErrTxDone), errors.Is(err, sql.ErrConnDone),
		errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF):
		return true
	}
	var ne net.Error
	if errors.As(err, &ne) {
		return true
	}
	c, ok := repo.(txAbortClassifier)
	return ok && c.IsTxAborting(err)
}
