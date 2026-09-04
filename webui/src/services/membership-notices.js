// The API stores current group membership, but no join/leave events.
// Keeping consecutive member lists lets this frontend infer those events.
// They are saved per account so changing views, refreshing the page
// or signing out and back in does not erase the notices already observed.
//
// Each notice carries the instant it was observed, which is what places it inside the conversation:
// without it a notice has no place of its own and can only be drawn after the last message,
// where it would keep sliding down as the chat grows.
// The instant is when this client noticed the change and not when it happened,
// the API records no event to read that from, but it never moves again,
// so the notice stays between the same two messages for good

const STORAGE_KEY = 'wasatext.membershipNotices'
const groups = new Map()
let sequence = 0

// Membership history is private to both the signed-in account and the group.
function mapKey(ownerId, chatId) {
	return `${ownerId}:${chatId}`
}

// Restore valid version-2 history while ignoring malformed records independently.
function read() {
	try {
		const parsed = JSON.parse(localStorage.getItem(STORAGE_KEY))
		// Version 1 saved no instant with a notice, so nothing says where those belong:
		// they are dropped rather than piled up at the end of every conversation
		if (!parsed || parsed.version !== 2 || !Array.isArray(parsed.groups)) return
		if (Number.isSafeInteger(parsed.sequence) && parsed.sequence >= 0) sequence = parsed.sequence

		// check one by one and save only the valid
		for (const saved of parsed.groups) {
			if (!saved || typeof saved.ownerId !== 'string' || typeof saved.chatId !== 'string') continue
			if (!Array.isArray(saved.members) || !Array.isArray(saved.notices)) continue
			const members = saved.members.filter((id) => typeof id === 'string')
			const notices = saved.notices.filter(
				(n) =>
					// exists
					n &&
					(n.type === 'joined' || n.type === 'left') &&
					typeof n.userId === 'string' &&
					typeof n.key === 'string' &&
					typeof n.date === 'string' &&
					!isNaN(new Date(n.date).getTime())
			)
			groups.set(mapKey(saved.ownerId, saved.chatId), {
				ownerId: saved.ownerId,
				chatId: saved.chatId,
				// Array to set, the inverse of write()
				members: new Set(members),
				notices,
			})
		}
	} catch {
		// Corrupt or unavailable storage starts with an empty history; the live UI still works
	}
}

// Persist the in-memory history, converting member Sets into JSON-compatible arrays.
function write() {
	try {
		localStorage.setItem(
			STORAGE_KEY,
			JSON.stringify({
				version: 2,
				sequence,
				groups: [...groups.values()].map((group) => ({
					ownerId: group.ownerId,
					chatId: group.chatId,
					members: [...group.members],
					notices: group.notices,
				})),
			})
		)
	} catch {
		// Storage may be disabled or full; keep the notices in memory for this page session
	}
}

// Record one inferred change with a stable key and the time this client observed it.
function addNotice(group, type, userId) {
	sequence += 1
	group.notices.push({
		type,
		userId,
		// Where the notice sits in the conversation, fixed once and never recomputed, frozen forever
		date: new Date().toISOString(),
		key: `${type}-membership-${sequence}-${userId}`,
	})
}

// Compare the latest membership snapshot with the previous one and return all notices.
// The first snapshot establishes the baseline, so existing members are not reported as new joins.
export function observeGroupMembers(ownerId, chatId, members) {
	const now = new Set(members)
	const key = mapKey(ownerId, chatId)
	let group = groups.get(key)

	if (!group) {
		group = { ownerId, chatId, members: now, notices: [] }
		groups.set(key, group)
		write()
		return group.notices
	}

	let changed = false
	for (const userId of now) {
		if (!group.members.has(userId)) {
			addNotice(group, 'joined', userId)
			changed = true
		}
	}
	for (const userId of group.members) {
		if (!now.has(userId)) {
			addNotice(group, 'left', userId)
			changed = true
		}
	}

	if (changed) {
		group.members = now
		write()
	}
	return group.notices
}

// First run
read()
