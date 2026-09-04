// Small presentation helpers shared by the list and the opened chat.

// How much of a message reaches its preview. The server cuts there (schemas.SnippetMaxChars),
// so this is not a limit the client applies: it is the one it has to recognise
const SNIPPET_MAX_CHARS = 50

// ---------- string ----------

// countChars counts the way the server does: grapheme clusters, not code points,
// so a family emoji is one character and not eleven.
// Intl.Segmenter is what matches schemas.CountChars;
// the spread is a fallback for a browser without it, and only overcounts the rare clusters
export function countChars(s) {
	const trimmed = (s || '').trim()
	if (!trimmed) return 0
	if (typeof Intl !== 'undefined' && Intl.Segmenter) {
		return [...new Intl.Segmenter().segment(trimmed)].length
	}
	return [...trimmed].length
}

// ---------- date ----------

// Dates come back as RFC 3339 in UTC, and Go trims trailing zeroes from the milliseconds,
// so the same instant can read '...19.45Z' or '...19Z'. Date parses both: never slice the string
function toDate(value) {
	const d = new Date(value)
	return isNaN(d.getTime()) ? null : d
}

// clockTime is the time inside a chat: the hour and minute of the message
export function clockTime(value) {
	const d = toDate(value)
	if (!d) return ''
	return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
}

// listTime is what the conversations list shows: the hour for today, the weekday for this week,
// the date before that, enough to place the chat without taking a whole row
export function listTime(value) {
	const d = toDate(value)
	if (!d) return ''

	const now = new Date()
	if (sameDay(d, now)) return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })

	const days = (now - d) / (1000 * 60 * 60 * 24)
	if (days < 7) return d.toLocaleDateString([], { weekday: 'short' })

	return d.toLocaleDateString([], { day: '2-digit', month: '2-digit', year: '2-digit' })
}

// dayLabel separates the messages of one day from the next inside a chat
export function dayLabel(value) {
	const d = toDate(value)
	if (!d) return ''

	const now = new Date()
	const startOf = (x) => new Date(x.getFullYear(), x.getMonth(), x.getDate())
	const days = Math.round((startOf(now) - startOf(d)) / (1000 * 60 * 60 * 24))
	if (days === 0) return 'Today'
	if (days === 1) return 'Yesterday'
	if (days < 7) return d.toLocaleDateString([], { weekday: 'long' })
	return d.toLocaleDateString([], { day: 'numeric', month: 'long', year: 'numeric' })
}

// sameDay says whether two messages belong under the same day separator
export function sameDay(a, b) {
	const da = toDate(a)
	const db = toDate(b)
	if (!da || !db) return false
	return (
		da.getDate() === db.getDate() &&
		da.getMonth() === db.getMonth() &&
		da.getFullYear() === db.getFullYear()
	)
}

// ---------- snippet ----------

// snippetText draws the preview of the conversations list.
// The server sends the head of the text, a 📷 standing for a photo, or both,
// and no snippet at all while the chat has nothing to preview
export function snippetText(snippet) {
	if (!snippet || !snippet.content) return ''
	// absent = undefined = false
	const emoji = snippet.content.emoji
	const text = previewText(snippet.content.text)
	if (emoji && text) return `${emoji} ${text}`
	if (emoji) return emoji
	return text
}

// A message longer than the cap reaches the client already cut, and the cut falls wherever the 50th character happens to be,
// mid-word more often than not.
// Without a mark reads as broken text rather than as the head of a longer message, so the "…" is added back here.
// A message of exactly 50 characters gets one it does not need;
// that is the cheaper mistake, since the alternative is refetching the whole message to find out
function previewText(text) {
	if (!text) return ''
	return countChars(text) >= SNIPPET_MAX_CHARS ? text + '…' : text
}
