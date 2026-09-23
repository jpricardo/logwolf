package main

import (
	"errors"
	"logwolf-toolbox/data"
	"net/http"
	"strings"
)

// net/rpc flattens every error the logger returns into its message, so errors.Is
// cannot see data.ErrLastOwner or mongo.ErrNoDocuments on this side of the wire.
// classifyRPCError reads the cause back out of the message in one place, so the
// handlers agree on which failures are the caller's.

type rpcErrorKind int

const (
	rpcErrInternal  rpcErrorKind = iota // not the caller's mistake
	rpcErrDuplicate                     // a unique index refused the write
	rpcErrNotFound                      // no document matched, or the id cannot name one
	rpcErrLastOwner                     // data.ErrLastOwner
)

func classifyRPCError(err error) rpcErrorKind {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "E11000"):
		return rpcErrDuplicate
	// A malformed id is not found either: no document can have it.
	case strings.Contains(msg, "no documents in result"), strings.Contains(msg, "not a valid ObjectID"),
		strings.Contains(msg, data.ErrKeyNotFound.Error()):
		return rpcErrNotFound
	case strings.Contains(msg, data.ErrLastOwner.Error()):
		return rpcErrLastOwner
	}
	return rpcErrInternal
}

var rpcErrorStatus = map[rpcErrorKind]int{
	rpcErrInternal:  http.StatusInternalServerError,
	rpcErrDuplicate: http.StatusConflict,
	rpcErrNotFound:  http.StatusNotFound,
	rpcErrLastOwner: http.StatusBadRequest,
}

// rpcErrorMessages says what to tell the caller for each kind of mistake a
// logger call can report. The logger's own wording names Mongo internals, so a
// kind left out keeps it only as a last resort.
type rpcErrorMessages map[rpcErrorKind]string

// projectNotFound covers the membership checks, whose only client mistake is a
// project id that names nothing.
var projectNotFound = rpcErrorMessages{rpcErrNotFound: "project not found"}

// logNotFound covers a log id that names nothing in the project, including one
// that belongs to another project.
var logNotFound = rpcErrorMessages{rpcErrNotFound: "log not found"}

// keyNotFound covers an API key id that names nothing, including, on revoke,
// one that belongs to another project.
var keyNotFound = rpcErrorMessages{rpcErrNotFound: "key not found"}

// rpcErrorJSON answers a failed logger call with the status its kind maps to.
// Anything unclassified is a 500: the logger or the database failed, not the
// caller.
func (app *Config) rpcErrorJSON(w http.ResponseWriter, err error, msgs rpcErrorMessages) {
	kind := classifyRPCError(err)
	if msg, ok := msgs[kind]; ok && kind != rpcErrInternal {
		err = errors.New(msg)
	}
	app.errorJSON(w, err, rpcErrorStatus[kind])
}
