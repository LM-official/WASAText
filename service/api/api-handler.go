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
	// My routes
	rt.router.POST("/session", rt.wrap(rt.doLogin))
	rt.router.PATCH("/me/username", rt.wrap(rt.authenticate(rt.setMyUserName)))
	rt.router.PATCH("/me/photo", rt.wrap(rt.authenticate(rt.setMyPhoto)))
	rt.router.GET("/users", rt.wrap(rt.authenticate(rt.getUsers)))
	rt.router.GET("/me/chats", rt.wrap(rt.authenticate(rt.getMyConversations)))
	rt.router.GET("/chats/:chatId", rt.wrap(rt.authenticate(rt.getConversation)))
	rt.router.POST("/chats/:chatId/messages", rt.wrap(rt.authenticate(rt.sendMessage)))
	rt.router.POST("/chats/:chatId/messages/forwards", rt.wrap(rt.authenticate(rt.forwardMessage)))
	rt.router.PUT("/chats/:chatId/messages/:messageId/comments/me", rt.wrap(rt.authenticate(rt.commentMessage)))
	rt.router.POST("/private-chats", rt.wrap(rt.authenticate(rt.createPrivateChat)))
	rt.router.POST("/groups", rt.wrap(rt.authenticate(rt.createGroup)))
	rt.router.PATCH("/groups/:groupId/name", rt.wrap(rt.authenticate(rt.setGroupName)))
	rt.router.PATCH("/groups/:groupId/photo", rt.wrap(rt.authenticate(rt.setGroupPhoto)))
	rt.router.POST("/groups/:groupId/members", rt.wrap(rt.authenticate(rt.addToGroup)))
	rt.router.DELETE("/groups/:groupId/members/me", rt.wrap(rt.authenticate(rt.leaveGroup)))

	// My helpers
	rt.router.GET("/photos/:photoId", rt.wrap(rt.authenticate(rt.getPhoto)))

	// Special routes
	rt.router.GET("/liveness", rt.liveness)

	return rt.router
}
