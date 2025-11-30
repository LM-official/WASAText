/*	QUESTO FILE VA LASCIATO COSI COME'.
	Definisce un'interfaccia custom "httpRouterHandler" che estende la firma standard degli
	handler http per inserire, come quarto parametro, un logger a cui sara possibile accedere direttamente.
	Questo significa che quando andiamo a scrivere i nostri handlers questi dovranno avere la firma:
		func genericHandler(http.ResponseWriter, *http. Request, httprouter.Params, reqcontext. RequestContext)
	ANZICHE
		func genericHandler(http.ResponseWriter, *http.Request, httprouter. Params)

	La funzione wrapper vera e propria (rt.wrap) si occupa di adattare questa firma estesa a quella standard.
	La chiamata a rt. wrap deve avvenire OGNI una volta che registriamo un handler su una rotta nel file
	service/api/api-handler.go. Ad esempio:
		rt. router. GET("/users 				---> It.wrap(rt. searchUsersHandler) <---)
		rt. router. GET ("/users/: userId", 	---> rt.wrap(rt.getUserHandler) <---)
*/
/*	È fornito dal professore e non va modificato.
	Definisce una firma custom per gli handler HTTP.
	Gli handler che genereremo devono rispettare quella firma.
*/

package api

import (
	"net/http"

	"github.com/MercuriLorenzo/WASAText/service/api/reqcontext"
	"github.com/gofrs/uuid"
	"github.com/julienschmidt/httprouter"
	"github.com/sirupsen/logrus"
)

// httpRouterHandler is the signature for functions that accepts a reqcontext.RequestContext in addition to those
// required by the httprouter package.
type httpRouterHandler func(http.ResponseWriter, *http.Request, httprouter.Params, reqcontext.RequestContext)

// wrap parses the request and adds a reqcontext.RequestContext instance related to the request.
func (rt *_router) wrap(fn httpRouterHandler) func(http.ResponseWriter, *http.Request, httprouter.Params) {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		reqUUID, err := uuid.NewV4()
		if err != nil {
			rt.baseLogger.WithError(err).Error("can't generate a request UUID")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		var ctx = reqcontext.RequestContext{
			ReqUUID: reqUUID,
		}

		// Create a request-specific logger
		ctx.Logger = rt.baseLogger.WithFields(logrus.Fields{
			"reqid":     ctx.ReqUUID.String(),
			"remote-ip": r.RemoteAddr,
		})

		// Call the next handler in chain (usually, the handler function for the path)
		fn(w, r, ps, ctx)
	}
}
