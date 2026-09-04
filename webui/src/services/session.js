import { reactive } from 'vue'

// The only global state of the app.
//
// doLogin answers with the user id and nothing else, and that id is both the identity of the user
// and the bearer token of every other request (doc/api.yaml: BearerAuth).
// It is kept in localStorage so a reload does not log anybody out,
// together with the username and the photo, which are shown in the interface before any request has had time to come back.
//
// The project ignores the security of this token on purpose (see doc/WASA_project.md,
// "Nota sulla Sicurezza"): there is no password to protect and no session to expire.

const KEY_ID = 'wasatext.userId'
const KEY_USERNAME = 'wasatext.username'
const KEY_PHOTO = 'wasatext.photo'

// A private window with storage disabled must not break the app: without localStorage the session simply does not survive a reload
function read(key) {
	try {
		return localStorage.getItem(key)
	} catch {
		return null
	}
}

function write(key, value) {
	try {
		if (value === null) localStorage.removeItem(key)
		else localStorage.setItem(key, value)
	} catch {
		// Nothing to do: the session lives in memory for as long as the page does
	}
}

export const session = reactive({
	userId: read(KEY_ID),
	username: read(KEY_USERNAME),
	photo: read(KEY_PHOTO),
})

// isLoggedIn is what the router guard asks. Only the id matters: it is the token
export function isLoggedIn() {
	// !! answers with a yes or a no, instead of handing the token itself back to the caller
	return !!session.userId
}

// setUser stores the User the API answered with, whichever endpoint it came from
// doLogin, getUser, setMyUserName and setMyPhoto all describe the same row
export function setUser(user) {
	session.userId = user.id
	session.username = user.username
	session.photo = user.photo
	write(KEY_ID, user.id)
	write(KEY_USERNAME, user.username)
	write(KEY_PHOTO, user.photo)
}

export function logout() {
	session.userId = null
	session.username = null
	session.photo = null
	write(KEY_ID, null)
	write(KEY_USERNAME, null)
	write(KEY_PHOTO, null)
}
