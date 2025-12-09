/*	In questo file, per ogni handler creato, dobbiamo registrare la rotta corrispondente.
	Qui non si implementa logica: solo associazione route-handler.
	Esempio: rt.router.GET("/users", rt.wrap(rt.searchUsersHandler))
*/

package api

import (
	"net/http"
)

// Handler returns an instance of httprouter.Router that handle APIs registered here
func (rt *_router) Handler() http.Handler {
	// Register routes
	rt.router.GET("/", rt.getHelloWorld)
	rt.router.GET("/context", rt.wrap(rt.getContextReply))

	// my routes
	rt.router.POST("/session", rt.wrap(rt.doLogin))
	rt.router.PATCH("/me/username", rt.wrap(rt.authenticate(rt.setMyUserName)))
	rt.router.GET("/users", rt.wrap(rt.authenticate(rt.searchUsers)))

	// Special routes
	rt.router.GET("/liveness", rt.liveness)

	return rt.router
}
