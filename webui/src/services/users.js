import { reactive } from 'vue'
import { getUser } from './api.js'

// The id -> user directory.
//
// Every read of the API hands out ids and not users: `message.user`, `comment.user` and the `members` of a chat are all a UserId,
// while the interface has to draw a username and a photo.
// getUser answers one id at a time, so this keeps what came back.
//
// The cache is reactive, so a template can read `directory[id]` and re-render by itself when the answer lands.
//
// The members of a chat arrive with their username and photo inside the answer itself,
// so a rename reaches the screen on the next call that returns them,
// e.g. the poll of an open chat, or the reply of the operation that just changed the group.
// What is left here is the one case no member list covers: a sender who wrote and then left,
// whose name is read once by id and then kept, since somebody outside the chat is not renaming himself in it

const directory = reactive({})

// One promise per id in flight: the same unknown sender appearing on 40 messages asks once
const inflight = new Map()

// remember stores a user the caller already has, so it never has to be asked for.
// The search results, the login answer and every User the API returns go through here
export function remember(user) {
	if (user && user.id) directory[user.id] = user
	return user
}

export function rememberAll(users) {
	for (const u of users) remember(u)
	return users
}

// resolve gives back the user of an id, asking the API only if it is not known yet.
// Private to this file: resolveMany is what the views call, one page of ids at a time.
// A user that cannot be read/deleted, or an id that names nobody is remembered as a placeholder
// rather than asked for again on every poll
async function resolve(userId) {
	if (!userId) return undefined
	if (directory[userId]) return directory[userId]
	if (inflight.has(userId)) return inflight.get(userId)

	const p = getUser(userId)
		.then((user) => remember(user))
		.catch(() => {
			// An id that names nobody is not going to start naming somebody: the placeholder stays
			return remember({ id: userId, username: 'unknown user', photo: null })
		})
		.finally(() => inflight.delete(userId))

	inflight.set(userId, p)
	return p
}

// resolveMany asks for the ids nothing is known about yet, in one pass, so a view can await once
// before it renders instead of resolving inside the template.
// A name already in the directory is never asked for again: the answer that carried it is the same answer that would carry a newer one
export async function resolveMany(userIds) {
	const unknown = [...new Set(userIds.filter((id) => id && !directory[id]))]
	await Promise.all(unknown.map(resolve))
}

// username is what the templates call: the name if it is known, a short form of the id while it is not,
// so a message never renders with an empty author.
// The short id is safe guard for resolve() errors.
// It never happens if all goes well
export function username(userId) {
	const u = directory[userId]
	if (u) return u.username
	return userId ? userId.slice(0, 8) : ''
}

export function clear() {
	for (const k of Object.keys(directory)) delete directory[k]
	inflight.clear()
}
