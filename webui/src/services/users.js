import { reactive, watch } from 'vue'
import { lookupUsers } from './api.js'
import { session } from './session.js'

// Chat membership and message authors are IDs. All displayed profiles live here
const directory = reactive({})
const inflight = new Map()
const versions = new Map()
let generation = 0

// Store a profile already received from the API and return it unchanged.
// Its version prevents an older lookup still in flight from overwriting this value
export function remember(user) {
	if (user && user.id) {
		directory[user.id] = user
		versions.set(user.id, (versions.get(user.id) || 0) + 1)
	}
	return user
}

// Store every profile in a response and return the original list
export function rememberAll(users) {
	for (const user of users) remember(user)
	return users
}

// Read the cached profile, or show a short ID and no photo while it is unknown.
// This only reads reactive state; it never sends a request
export function profile(userId) {
	return directory[userId] || { id: userId, username: userId ? userId.slice(0, 8) : '', photo: null }
}

// Return the cached username, with the same short-ID fallback as profile()
export function username(userId) {
	return profile(userId).username
}

// Resolve unique IDs in batches of at most 100, sharing lookups already in flight.
// With refresh=true, also request profiles that are already cached.
// Ignore replies from an old session or cache generation and preserve newer local values.
// Return false if any awaited batch fails or belongs to an old session;
// failed requests leave cached profiles intact and can be retried by the next call
async function resolveProfiles(userIds, refresh) {
	const ids = [...new Set(userIds.filter(Boolean))]
	const pending = ids.filter((id) => (refresh || !directory[id]) && !inflight.has(id))
	const epoch = generation
	const owner = session.userId
	for (let i = 0; i < pending.length; i += 100) {
		const batch = pending.slice(i, i + 100)
		const before = new Map(batch.map((id) => [id, versions.get(id)]))
		const promise = lookupUsers(batch)
			.then((users) => {
				if (generation !== epoch || session.userId !== owner) return false
				const found = new Map(users.map((user) => [user.id, user]))
				for (const id of batch) {
					// A profile explicitly remembered since this read started is newer
					if (versions.get(id) !== before.get(id)) continue
					remember(found.get(id) || { id, username: 'unknown user', photo: null })
				}
				return true
			})
			.catch(() => false) // Keep cached profiles; subsequent calls can retry
			.finally(() => {
				for (const id of batch) {
					if (inflight.get(id) === promise) inflight.delete(id)
				}
			})
		for (const id of batch) inflight.set(id, promise)
	}
	const requests = [...new Set(ids.map((id) => inflight.get(id)).filter(Boolean))]
	const results = await Promise.all(requests)
	return results.every(Boolean)
}

// Fetch profiles missing from the directory, reusing cached profiles and pending lookups.
// Return whether all awaited batches succeeded
export function resolveMany(userIds) {
	return resolveProfiles(userIds, false)
}

// Refresh the requested profiles even when cached, sharing any lookup already in flight.
// Return whether all awaited batches succeeded
export function refreshMany(userIds) {
	return resolveProfiles(userIds, true)
}

// Empty the profile cache and its request bookkeeping when the session changes.
// Advancing the generation makes outstanding replies harmless; it does not cancel them
export function clear() {
	generation++
	for (const id of Object.keys(directory)) delete directory[id]
	inflight.clear()
	versions.clear()
}

// Includes automatic logout after a 401, not just the profile page's logout button
watch(() => session.userId, clear, { flush: 'sync' })
