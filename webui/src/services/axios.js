import axios from "axios";
import { session, isLoggedIn, logout } from "./session.js";

const instance = axios.create({
	// __API_URL__ is defined in vite.config.js and is used in the evaluation: do not replace it
	baseURL: __API_URL__,
	// Time enough for anything but a photo:
	// doc/api.yaml allows 30 MiB per upload, and getPhoto serves the same bytes back,
	// so the calls that carry those bytes pass PHOTO_TIMEOUT per request instead
	timeout: 1000 * 20
});

// What a call carrying a photo waits instead of the default above.
// A photo is 30 MiB at most (schemas.MaxPhotoBytes), which does not fit in 20 seconds
export const PHOTO_TIMEOUT = 1000 * 120;

// How often views refresh data while they are visible.
export const POLL_MS = 3000;

// Every operation but doLogin is authenticated, and the token is the user id doLogin answered with (doc/WASA_project.md, "Meccanismo di Autenticazione"):
// one interceptor puts it on every request instead of each call remembering to
instance.interceptors.request.use((config) => {
	if (isLoggedIn()) {
		config.headers.Authorization = `Bearer ${session.userId}`;
	}
	return config;
});

// A 401 means the stored token names nobody the server knows: the database was recreated, or the value was edited by hand.
// Keeping it would make every following request fail the same way, so the session is dropped and the page is sent back to the login view.
// The hash is written directly rather than through the router:
// the router imports the views, which import this file, so importing it back here would close the cycle.
// createWebHashHistory makes the two equivalent anyway
instance.interceptors.response.use(
	(response) => response,
	(error) => {
		if (error.response && error.response.status === 401 && isLoggedIn()) {
			logout();
			window.location.hash = "#/login";
		}
		return Promise.reject(error);
	}
);

export default instance;
