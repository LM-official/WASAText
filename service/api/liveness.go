/* LASCIO COSI COM'E
Questo file definisce l'handler per l'endpoint di /liveness usato per verificare lo stato di
salute del servizio.
Questa funzione deve, come tutti gli altri handler, attaccata ad un path nel file api-handler.go
E' GIA STATO FATTO DAL PROF. Lasciare (o re-inserire) quella riga li:
	rt. router. GET ("/liveness", It.liveness)
*/

// Contiene l'handler per GET /liveness. Non va modificato.

package api

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
)

// Liveness is an HTTP handler that checks the API server status
// If the server cannot serve requests (e.g., some resources are not ready),
// this should reply with HTTP Status 500. Otherwise, with HTTP Status 200
func (rt *_router) liveness(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	/* Example of liveness check:
	if err := rt.DB.Ping(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}*/
}
