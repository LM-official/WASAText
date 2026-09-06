import axios, { PHOTO_TIMEOUT } from './axios.js'

// One function per operationId of doc/api.yaml, so that no component ever builds a URL
// or remembers the shape of a multipart body.
// Everything above this file works with objects.

// ---------- errors ----------

// errorMessage turns any failure into the one line the user reads.
// Every 4xx/5xx of this API carries schemas.Error, `{"code":..,"message":".."}`,
// and the message is the only text meant for the client: the technical cause stays in the server log.
// The router answers before the handlers for a 404/405 and those bodies are plain text, so they fall back to the axios message
export function errorMessage(e) {
	const data = e && e.response && e.response.data
	if (data && typeof data.message === 'string') return data.message
	if (e && e.code === 'ECONNABORTED') return 'The server took too long to answer'
	if (e && e.request && !e.response) return 'Cannot reach the server'
	return e ? e.toString() : 'Unknown error'
}

// emptyOn404 reads the no-results convention of this API.
// getUsers and getMyConversations answer 404 and never 200 with an empty list
// (tests/endpoints.md §3, §13), so for those two an empty result is not a failure
async function emptyOn404(promise, fallback) {
	try {
		return await promise
	} catch (e) {
		if (e.response && e.response.status === 404) return fallback
		throw e
	}
}

// ---------- login ----------

// doLogin: 201 when the username is new and the user is registered, 200 when it already existed.
// Both answer with the id, which is also the bearer token of every other call
export async function doLogin(username) {
	const res = await axios.post('/session', { username })
	return res.data.id
}

// ---------- users ----------

// getUser resolves one ID
// Message senders, comment authors and chat members all travel as ids
export async function getUser(userId) {
	const res = await axios.get(`/users/${userId}`)
	return res.data
}

// lookupUsers resolves a batch of profiles.
// The server answers 200 with an empty list for unknown ids,
// so the caller can batch without checking
export async function lookupUsers(ids) {
	const res = await axios.post('/users_lookup', { ids })
	return res.data.users
}

// getUsers: prefix search, capped at 20 by the server
export async function getUsers(prefix) {
	return emptyOn404(
		axios.get('/users', { params: { username: prefix } }).then((r) => r.data.users),
		[]
	)
}

export async function setMyUserName(username) {
	const res = await axios.put('/me/username', { username })
	return res.data
}

export async function setMyPhoto(file) {
	const fd = new FormData()
	fd.append('photoFile', file)
	const res = await axios.put('/me/photo', fd, { timeout: PHOTO_TIMEOUT })
	return res.data
}

// ---------- chats ----------

// Group metadata has no message-read side effects
export async function getGroup(groupId) {
	const res = await axios.get(`/groups/${groupId}`)
	return res.data
}

// getMyConversations: the homepage list, most recently active first, each with the preview of its last message.
// A chat nobody has written in carries no snippet
export async function getMyConversations() {
	return emptyOn404(
		axios.get('/me/chats').then((r) => r.data.chats),
		[]
	)
}

// getConversation: the opened chat, its members and a page of its messages, newest first.
// Reading it also marks the chat read for the caller, which is what turns one checkmark into two for everybody else
export async function getConversation(chatId) {
	const res = await axios.get(`/chats/${chatId}`)
	return res.data
}

// ---------- messages ----------

// sendMessage: multipart, `text` as a plain form field and `photoFile` as a file, at least one of the two.
// Content-Type is left to the browser, which is the only one that can write the boundary
//
// `replyTo` is what makes the message an answer, and it is a field of this same call rather than an endpoint of its own:
// the message it names must belong to this chat, or the server answers 404
export async function sendMessage(chatId, { text, file, replyTo }) {
	const fd = new FormData()
	if (text) fd.append('text', text)
	if (file) fd.append('photoFile', file)
	if (replyTo) fd.append('replyTo', replyTo)
	const res = await axios.post(`/chats/${chatId}/messages`, fd, { timeout: PHOTO_TIMEOUT })
	return res.data
}

// forwardMessage: the id in the body names the source message, the id in the URL the destination chat.
// The copy gets a new id, the caller as its sender, and no comments
export async function forwardMessage(chatId, messageId) {
	const res = await axios.post(`/chats/${chatId}/forwards`, { messageId })
	return res.data
}

// deleteMessage: only its sender may, and it answers 204 with no body
export async function deleteMessage(chatId, messageId) {
	await axios.delete(`/chats/${chatId}/messages/${messageId}`)
}

// ---------- comments (reactions) ----------

// commentMessage: sets the caller's one reaction and answers with the whole updated message,
// so the copy already on screen can be replaced in one reply
export async function commentMessage(chatId, messageId, emoji) {
	const res = await axios.put(`/chats/${chatId}/messages/${messageId}/comments/me`, { emoji })
	return res.data
}

// uncommentMessage: removes the caller's own reaction, 204 with no body.
// No comment id travels: (message, caller) already names at most one
export async function uncommentMessage(chatId, messageId) {
	await axios.delete(`/chats/${chatId}/messages/${messageId}/comments/me`)
}

// ---------- private chats and groups ----------

// createPrivateChat: 201 for a new chat, 200 for the one that was already there.
// Both return the chat metadata and member IDs (ChatWithMembers).
// The pair is one row either way, so calling it twice can never open two chats
export async function createPrivateChat(otherUserId) {
	const res = await axios.post('/private_chats', { id: otherUserId })
	return res.data
}

// createGroup: multipart with a JSON `data` part and an optional `photoFile`.
// `data` has to be appended as a string:
// a blob would be sent with a filename, which makes it a file part,
// which the server's r.FormValue("data") cannot see (-> 400 invalid data part).
// Without a photo the group starts from the default.
// The creator must not be in `members`: the server adds it.
// Returns the chat metadata and member IDs, including the creator (ChatWithMembers)
export async function createGroup({ name, members, file }) {
	const fd = new FormData()
	fd.append('data', JSON.stringify({ name, members }))
	if (file) fd.append('photoFile', file)
	const res = await axios.post('/groups', fd, { timeout: PHOTO_TIMEOUT })
	return res.data
}

// setGroupName: every member may rename, a group has no owner.
// The reply is the chat alone, without its members and without its messages,
// so only the base is refreshed from it.
// A group name has no UNIQUE behind it, so renaming to the name it already has is a 200
export async function setGroupName(groupId, name) {
	const res = await axios.put(`/groups/${groupId}/name`, { name })
	return res.data
}

// setGroupPhoto: the single `photoFile` field, and every member may replace it, as for the name.
// The reply is the same chat-alone shape setGroupName answers with.
// A photo is written once and never rewritten: the same bytes uploaded again get a new id,
// so the URL always changes and AuthPhoto fetches the new picture instead of showing the one photos.js already cached under the old id
export async function setGroupPhoto(groupId, file) {
	const fd = new FormData()
	fd.append('photoFile', file)
	const res = await axios.put(`/groups/${groupId}/photo`, fd, { timeout: PHOTO_TIMEOUT })
	return res.data
}

// addToGroup answers with the group and its whole member list after the write,
// so the length is what the table holds and not what the request asked for
export async function addToGroup(groupId, members) {
	const res = await axios.post(`/groups/${groupId}/members`, { members })
	return res.data
}

// leaveGroup: `me` is the caller, and removing anybody else is not an operation this API has
export async function leaveGroup(groupId) {
	await axios.delete(`/groups/${groupId}/members/me`)
}
