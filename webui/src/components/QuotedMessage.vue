<script>
import { session } from '../services/session.js'
import { username } from '../services/users.js'

// The message an answer points at, drawn small above it.
//
// The API answers with `replyTo` as an id and nothing more, the same way it hands out user and chat ids:
// what it names is a message of this same chat, so the client already holds it in the page it read.
// When it does not, the quote is older than the 500-message page, or its message has just been deleted,
// there is nothing to draw and the component says so rather than showing an id.
export default {
	// Values received from a parent
	props: {
		// The resolved message, or null when the page does not hold it
		message: { type: Object, default: null },
		// A quote inside a green outgoing bubble needs a lighter bar than one on white
		onOwnBubble: { type: Boolean, default: false },
	},
	// Update parents
	emits: ['jump'],
	// Values derived from reactive values
	computed: {
		author() {
			if (!this.message) return ''
			return this.message.user === session.userId ? 'You' : username(this.message.user)
		},
		preview() {
			if (!this.message) return ''
			const c = this.message.content || {}
			if (c.photo && c.text) return `📷 ${c.text}`
			if (c.photo) return '📷 Photo'
			return c.text || ''
		},
	},
}
</script>

<template>
  <div
    :class="['quote', onOwnBubble ? 'quote-own' : '', message ? 'quote-clickable' : 'quote-gone']"
    :role="message ? 'button' : null"
    @click="message && $emit('jump', message.id)"
  >
    <template v-if="message">
      <div class="quote-author">{{ author }}</div>
      <div class="quote-preview">{{ preview }}</div>
    </template>
    <div v-else class="quote-preview fst-italic">Original message not available</div>
  </div>
</template>

<style scoped>
.quote {
	border-left: 3px solid #0d6efd;
	background-color: rgba(13, 110, 253, 0.07);
	border-radius: 0.35rem;
	padding: 0.2rem 0.45rem;
	margin-bottom: 0.25rem;
	font-size: 0.8rem;
	overflow: hidden;
}

/* On the sender's own green bubble the blue bar reads as a second colour, so it borrows the bubble */
.quote-own {
	border-left-color: #1a7f37;
	background-color: rgba(26, 127, 55, 0.1);
}

.quote-gone {
	border-left-color: #adb5bd;
	background-color: rgba(0, 0, 0, 0.04);
}

.quote-clickable {
	cursor: pointer;
}

.quote-author {
	font-weight: 600;
	color: #0d6efd;
}

.quote-own .quote-author {
	color: #1a7f37;
}

.quote-preview {
	color: #495057;
	white-space: nowrap;
	overflow: hidden;
	text-overflow: ellipsis;
}
</style>
