package api

// The api layer answers in JSON on success and on failure alike:
// these helpers are the only place that writes a response body,
// so a handler never repeats the decode / validate / content-type / status / encode sequence

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/MercuriLorenzo/WASAText/service/api/reqcontext"
	"github.com/MercuriLorenzo/WASAText/service/schemas"
)

// Validator is anything that can check itself against its own schema rules:
// every request type of the schemas package implements it
type validator interface {
	IsValid() error
}

// decodeAndValidate fills req from the JSON body of the request and checks it
// A malformed body and a broken rule are both a 400 for the client, so they share one return
func decodeAndValidate(r *http.Request, req validator) error {
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		return fmt.Errorf("malformed JSON body: %w", err)
	}

	return req.IsValid()
}

// unmarshalAndValidate fills req from data and checks it
// The JSON of a multipart request is a part and not the body, so it is already read when it gets here
func unmarshalAndValidate(data []byte, req validator) error {
	if err := json.Unmarshal(data, req); err != nil {
		return fmt.Errorf("malformed JSON part: %w", err)
	}

	return req.IsValid()
}

// writeJSON replies with body encoded as JSON and the given status code
func writeJSON(w http.ResponseWriter, ctx reqcontext.RequestContext, code int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	if err := json.NewEncoder(w).Encode(body); err != nil {
		// The status line is already on the wire: the reply cannot be fixed, only logged
		ctx.Logger.WithError(err).Error("cannot encode the response body")
	}
}

// writeError replies with the Error object of doc/api.yaml and logs cause
// Cause stays in the server logs: message is the only text the client reads
// e.g. an internal failure never leaks its details outside
func writeError(w http.ResponseWriter, ctx reqcontext.RequestContext, code int, message string, cause error) {
	if cause != nil {
		ctx.Logger.WithError(cause).Error(message)
	} else {
		ctx.Logger.Error(message)
	}

	writeJSON(w, ctx, code, schemas.Error{Code: code, Message: message})
}
