/*	UN FILE PER OGNI HANDLER
	Questo file contiene un esempio di handler http che implementa l'interfaccia wrapper definita
	dal prof nel file api-context-wrapper.go.
	Lascia intendere che anche noi dobbiamo implementare tale interfaccia e che dobbiamo fare un
	file diverso per ogni nostro handler. Ad esempio avremo:
		search-users. go 	(per l'operationId searchUsers)
		get-user. go 		(per l'operationId getUser)
		update-user.go		(per l'operationId updateUser)

	(stessa cosa avviene nel file get-hello-world.go)
*/

package api

import (
	"github.com/MercuriLorenzo/WASAText/service/api/reqcontext"
	"github.com/julienschmidt/httprouter"
	"net/http"
)

// getContextReply is an example of HTTP endpoint that returns "Hello World!" as a plain text. The signature of this
// handler accepts a reqcontext.RequestContext (see httpRouterHandler).
func (rt *_router) getContextReply(w http.ResponseWriter, r *http.Request, ps httprouter.Params, ctx reqcontext.RequestContext) {
	w.Header().Set("content-type", "text/plain")
	_, _ = w.Write([]byte("Hello World!"))
}
