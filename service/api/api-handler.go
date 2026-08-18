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
	rt.router.GET("/context", rt.wrap(rt.getContextReply))

	// My routes
	rt.router.POST("/session", rt.wrap(rt.doLogin))
	rt.router.PATCH("/me/username", rt.wrap(rt.authenticate(rt.setMyUserName)))
	rt.router.PATCH("/me/photo", rt.wrap(rt.authenticate(rt.setMyPhoto)))
	rt.router.GET("/users", rt.wrap(rt.authenticate(rt.getUsers)))
	rt.router.GET("/photos/:photoId", rt.wrap(rt.authenticate(rt.getPhoto)))
	rt.router.POST("/private_chats", rt.wrap(rt.authenticate(rt.createPrivateChat)))
	rt.router.POST("/groups", rt.wrap(rt.authenticate(rt.createGroup)))
	rt.router.PATCH("/groups/:chatId/name", rt.wrap(rt.authenticate(rt.setGroupName)))

	// Special routes
	rt.router.GET("/liveness", rt.liveness)

	return rt.router
}
