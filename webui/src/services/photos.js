import axios, { PHOTO_TIMEOUT } from './axios.js'

// Photos cannot be reached with a plain <img src>.
//
// GET /photos/{photoId} is behind the same authenticate wrapper as every other operation (service/api/api-handler.go),
// and the browser sends no Authorization header when it loads an <img>:
// the request would come back 401 and the picture would never appear.
// So the bytes are fetched like any other call, with the interceptor putting the token on,
// and turned into an object URL that an <img> can use.
//
// The cache never expires.
// A photo id names one immutable file: uploading again writes a new id
// and the server collects the old file, so the bytes behind an id can never change under the cache.

const cache = new Map() // '/photos/<uuid>' -> Promise<objectURL>

// photoSrc gives back a URL an <img> can show, for a `/photos/<uuid>` the API answered with
export function photoSrc(url) {
	if (!url) return Promise.resolve(null)
	if (cache.has(url)) return cache.get(url)

	const p = axios
		.get(url, { responseType: 'blob', timeout: PHOTO_TIMEOUT })
		.then((res) => URL.createObjectURL(res.data))
		.catch((e) => {
			// A photo that cannot be read must not be retried on every re-render, but it must not be
			// remembered as missing forever either: a lost connection is not a deleted file
			cache.delete(url)
			throw e
		})

	cache.set(url, p)
	return p
}

// revokeAll releases every object URL.
// Called on logout: the blobs belong to the session that fetched them, and the next user has its own token
export function revokeAll() {
	for (const p of cache.values()) {
		p.then((objectURL) => URL.revokeObjectURL(objectURL)).catch(() => {
			// Nothing to revoke for a photo that never loaded
		})
	}
	cache.clear()
}
