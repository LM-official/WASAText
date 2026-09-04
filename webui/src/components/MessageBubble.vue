<script>
import { session } from '../services/session.js'
import { username } from '../services/users.js'
import { clockTime } from '../services/format.js'
import AuthPhoto from './AuthPhoto.vue'
import EmojiPicker from './EmojiPicker.vue'
import QuotedMessage from './QuotedMessage.vue'

// One message: its content, when it was sent, its delivery state,
// its reactions and what can be done to it.
export default {
	// Reusable components
	components: { AuthPhoto, EmojiPicker, QuotedMessage },
	// Values received from a parent
	props: {
		message: { type: Object, required: true },
		// The sender's name is drawn only in a group, and only on incoming messages:
		// in a private chat the other member is already the title of the whole conversation
		isGroup: { type: Boolean, default: false },
		// The message this one answers, already resolved from the page by the parent.
		// Null both when this message answers nothing and when the answered one is not in the page
		repliedTo: { type: Object, default: null },
	},
	// Update parents
	emits: ['react', 'unreact', 'forward', 'delete', 'reply', 'jump'],
	// Reactive state for each component instance
	data() {
		return {
			pickerOpen: false,
		}
	},
	// Values derived from reactive values
	computed: {
		mine() {
			return this.message.user === session.userId
		},
		senderName() {
			return username(this.message.user)
		},
		time() {
			return clockTime(this.message.date)
		},
		text() {
			return this.message.content ? this.message.content.text : null
		},
		photo() {
			return this.message.content ? this.message.content.photo : null
		},
		// The checkmarks of the project spec, and they belong to the sender alone:
		// one for a message every recipient has in its conversation list, two once every one of them has opened it.
		// `state` is a fact about the members of the chat and not about who is looking,
		// so it reads the same for everybody, which is why it is only drawn on what the caller sent
		ticks() {
			if (!this.mine) return null
			return this.message.state === 'read' ? '✓✓' : '✓'
		},
		ticksTitle() {
			return this.message.state === 'read'
				? 'Read by everyone in the chat'
				: 'Received, not read by everyone yet'
		},
		// The caller has at most one reaction per message: (message, user) names one row
		myComment() {
			return (this.message.comments || []).find((c) => c.user === session.userId) || null
		},
		// Reactions grouped by emoji, each carrying who posted it:
		// the project spec asks for the reaction and the names of the users behind it
		reactions() {
			const groups = new Map()
			for (const c of this.message.comments || []) {
				if (!groups.has(c.emoji)) groups.set(c.emoji, [])
				groups.get(c.emoji).push(c.user)
			}
			return [...groups.entries()].map(([emoji, users]) => ({
				emoji,
				users,
				names: users.map((u) => username(u)).join(', '),
				mine: users.includes(session.userId),
			}))
		},
	},
	// Functions used by components
	methods: {
		togglePicker() {
			this.pickerOpen = !this.pickerOpen
		},
		closePicker() {
			this.pickerOpen = false
		},
		react(emoji) {
			this.closePicker()
			this.$emit('react', { message: this.message, emoji })
		},
		unreact() {
			this.closePicker()
			this.$emit('unreact', { message: this.message })
		},
		// Clicking a reaction pill: the caller's own one comes off, anybody else's is adopted
		toggleReaction(group) {
			if (group.mine) this.unreact()
			else this.react(group.emoji)
		},
	},
}
</script>

<template>
  <div :class="['message-row d-flex mb-2', mine ? 'justify-content-end' : 'justify-content-start']">
    <div class="message-block">
      <div :class="['message-bubble', mine ? 'message-mine' : 'message-theirs']">
        <!-- Who wrote it: only in a group, and only when it is not the caller -->
        <div v-if="isGroup && !mine" class="message-sender">{{ senderName }}</div>

        <!-- What it answers. `replyTo` is on the message whether or not the page holds what it names,
			so the quote is drawn either way and says when it cannot be resolved -->
        <QuotedMessage
          v-if="message.replyTo"
          :message="repliedTo"
          :on-own-bubble="mine"
          @jump="$emit('jump', $event)"
        />

        <AuthPhoto
          v-if="photo"
          :src="photo"
          alt="Message photo"
          fluid
          :rounded="false"
          class="mb-1"
        />

        <div v-if="text" class="message-text">{{ text }}</div>

        <div class="message-meta">
          <span>{{ time }}</span>
          <span v-if="ticks" class="message-ticks" :title="ticksTitle">{{ ticks }}</span>
        </div>
      </div>

      <!-- The reactions and their authors -->
      <div v-if="reactions.length" :class="['reactions', mine ? 'justify-content-end' : '']">
        <button
          v-for="group in reactions"
          :key="group.emoji"
          type="button"
          :class="['reaction-pill', { 'reaction-mine': group.mine }]"
          :title="group.names"
          @click="toggleReaction(group)"
        >
          <span>{{ group.emoji }}</span>
          <span class="reaction-names">{{ group.names }}</span>
        </button>
      </div>

      <!-- What can be done to it. Deleting is offered only to the sender:
	   		the server answers 403 to anybody else, so a button there would be a promise the API does not keep -->
      <div :class="['message-actions', mine ? 'justify-content-end' : '']">
        <button
          type="button"
          class="btn btn-sm btn-link"
          @click="$emit('reply', { message })"
        >
          Reply
        </button>
        <button type="button" class="btn btn-sm btn-link" @click="togglePicker">React</button>
        <button
          type="button"
          class="btn btn-sm btn-link"
          @click="$emit('forward', { message })"
        >
          Forward
        </button>
        <button
          v-if="mine"
          type="button"
          class="btn btn-sm btn-link text-danger"
          @click="$emit('delete', { message })"
        >
          Delete
        </button>
      </div>

      <EmojiPicker
        v-if="pickerOpen"
        :mine="myComment ? myComment.emoji : null"
        class="mt-1"
        @pick="react"
        @remove="unreact"
      />
    </div>
  </div>
</template>

<style scoped>
.message-block {
	display: flex;
	flex-direction: column;
	align-items: flex-start;
	max-width: min(38rem, 80%);
}

.message-row.justify-content-end .message-block {
	align-items: flex-end;
}

.message-bubble {
	max-width: 100%;
	padding: 0.4rem 0.6rem;
	border-radius: 0.75rem;
	box-shadow: 0 1px 1px rgba(0, 0, 0, 0.08);
	word-wrap: break-word;
}

.message-mine {
	background-color: #d9fdd3;
	border-top-right-radius: 0.15rem;
}

.message-theirs {
	background-color: var(--bs-body-bg);
	border-top-left-radius: 0.15rem;
}

.message-sender {
	font-size: 0.78rem;
	font-weight: 600;
	color: #0d6efd;
	margin-bottom: 0.1rem;
}

.message-text {
	white-space: pre-wrap;
	word-break: break-word;
}

.message-meta {
	display: flex;
	align-items: center;
	justify-content: flex-end;
	gap: 0.25rem;
	font-size: 0.7rem;
	color: #667781;
	margin-top: 0.1rem;
}

.message-ticks {
	letter-spacing: -0.15em;
	color: #53bdeb;
	font-weight: 700;
}

.reactions,
.message-actions {
	display: flex;
	flex-wrap: wrap;
	gap: 0.25rem;
	margin-top: 0.15rem;
}

.reaction-pill {
	display: inline-flex;
	align-items: center;
	gap: 0.25rem;
	border: 1px solid #dee2e6;
	background-color: #fff;
	border-radius: 1rem;
	padding: 0.05rem 0.5rem;
	font-size: 0.75rem;
	cursor: pointer;
	max-width: 14rem;
}

.reaction-mine {
	background-color: #cfe2ff;
	border-color: #9ec5fe;
}

.reaction-names {
	color: #495057;
	overflow: hidden;
	text-overflow: ellipsis;
	white-space: nowrap;
}

/* The actions stay out of the way until the message is pointed at,
	so a long chat is not a wall of buttons.
	They are always shown on a touch screen, where there is no hover */
.message-actions {
	visibility: hidden;
}

.message-row:hover .message-actions,
.message-row:focus-within .message-actions {
	visibility: visible;
}

@media (hover: none) {
	.message-actions {
		visibility: visible;
	}
}

.message-actions .btn-link {
	padding: 0 0.25rem;
	font-size: 0.75rem;
	text-decoration: none;
}
</style>
