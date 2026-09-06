<script>
import {
	getConversation,
	sendMessage,
	forwardMessage,
	deleteMessage,
	commentMessage,
	uncommentMessage,
	errorMessage,
} from '../services/api.js'
import { POLL_MS } from '../services/axios.js'
import { session } from '../services/session.js'
import { resolveMany, refreshMany, username } from '../services/users.js'
import { observeGroupMembers } from '../services/membership-notices.js'
import { dayLabel, sameDay } from '../services/format.js'
import AuthPhoto from '../components/AuthPhoto.vue'
import MessageBubble from '../components/MessageBubble.vue'
import MessageComposer from '../components/MessageComposer.vue'
import ForwardDialog from '../components/ForwardDialog.vue'

// The opened chat.
//
// getConversation gives the chat, its members and a page of its messages, newest first.
// It is also what marks the chat read for the caller, which is what turns the sender's single checkmark into a double one,
// so opening a chat is a write as much as a read.
//
// The messages are reversed for display: the newest at the bottom, where a chat is read,
// and the view scrolls there on open. The API's own order is left untouched.
// When a membership notice was observed, as a number the sort and the merge below can compare.
// A notice saved without a readable instant has no place of its own and stays at the end,
// which is where the page used to draw every one of them
function noticeTime(notice) {
	const time = new Date(notice.date).getTime()
	return isNaN(time) ? Infinity : time
}

export default {
	// Reusable components
	components: { AuthPhoto, MessageBubble, MessageComposer, ForwardDialog },
	// Reactive state for each component instance
	data() {
		return {
			chat: null,
			loading: true,
			errormsg: null,
			actionError: null,
			sending: false,
			forwarding: null,
			timer: null,
			// A poll must not steal the scroll from someone reading further up
			stickToBottom: true,
			// Membership changes inferred and saved by the frontend
			membershipNotices: [],
			// The message the composer is answering, or null. Cleared once the answer is written,
			// so a refused send keeps the quote and the text the user already typed
			replyingTo: null,
			// The message the reader was just sent to by clicking a quote, briefly highlighted
			highlighted: null,
		}
	},
	// Values derived from reactive values
	computed: {
		chatId() {
			return this.$route.params.chatId
		},
		isGroup() {
			return this.chat && this.chat.chatType === 'group'
		},
		// Oldest first for the screen. The server answers newest first, which is the order it is specified in,
		// and reversing here leaves that untouched
		orderedMessages() {
			if (!this.chat) return []
			return [...this.chat.messages].reverse()
		},
		// Every message of the page by id, which is how a `replyTo` becomes the message it names.
		// The API answers with the id alone, the same way it hands out user and chat ids,
		// and what it names always belongs to this chat, so the page already holds it,
		// unless it is older than the 500 the page carries or it has just been deleted
		messagesById() {
			const byId = new Map()
			if (this.chat) for (const m of this.chat.messages) byId.set(m.id, m)
			return byId
		},
		// The messages, the day separators and the membership notices in one list,
		// so the template walks a single sequence instead of deciding what goes between two bubbles.
		//
		// A saved notice is placed at the instant it was observed, between the two messages it fell between:
		// appending them all at the end instead would draw a join from last week under today's messages,
		// and move it down again with every message written after it
		timeline() {
			const messages = this.orderedMessages
			// The API records no departure: leaveGroup answers 204 and takes the membership row with it.
			// A sender who is no longer a member therefore gets a notice after their last message.
			// Build that messageId -> userId lookup directly; the API order is newest first,
			// so the first message found for each departed sender is the one the notice follows.
			const after = new Map()
			if (this.isGroup) {
				const members = new Set(this.chat.members)
				const noticed = new Set(
					this.membershipNotices.filter((item) => item.type === 'left').map((item) => item.userId)
				)
				for (const m of this.chat.messages) {
					if (!members.has(m.user) && !noticed.has(m.user)) {
						after.set(m.id, m.user)
						noticed.add(m.user)
					}
				}
			}
			const notices = [...this.membershipNotices].sort((a, b) => noticeTime(a) - noticeTime(b))

			const items = []
			// The date of the last item that carries one:
			// a notice opens a day of its own the same way a message does,
			// so a join is never drawn under the separator of the wrong day
			let day = null
			const push = (date, item) => {
				if (date && (!day || !sameDay(date, day))) {
					items.push({ type: 'day', key: `day-${item.key}`, label: dayLabel(date) })
				}
				if (date) day = date
				items.push(item)
			}

			let next = 0
			// Everything observed before this message goes above it
			const noticesUntil = (time) => {
				while (next < notices.length && noticeTime(notices[next]) <= time) {
					const notice = notices[next++]
					push(isFinite(noticeTime(notice)) ? notice.date : null, notice)
				}
			}

			for (const m of messages) {
				noticesUntil(new Date(m.date).getTime())
				push(m.date, { type: 'message', key: m.id, message: m })
				// A departure read from the messages themselves belongs right after the last one its author wrote,
				// which is the only instant known for it
				const userId = after.get(m.id)
				if (userId) items.push({ type: 'left', key: `left-${m.id}-${userId}`, userId })
			}
			// Whatever was observed after the newest message, which is where a change seen right now lands
			noticesUntil(Infinity)
			return items
		},
	},
	// Run when a reactive value changes
	watch: {
		// Changing only the parameter of the same route does not remount the component
		// (doc/WASA_project.md §11), so the chat has to be reloaded by hand
		chatId: {
			immediate: true,
			handler() {
				this.chat = null
				this.loading = true
				this.errormsg = null
				this.actionError = null
				this.stickToBottom = true
				this.membershipNotices = []
				this.replyingTo = null
				this.highlighted = null
				this.load()
			},
		},
	},
	// Run after component is shown
	mounted() {
		this.timer = setInterval(this.poll, POLL_MS)
		document.addEventListener('visibilitychange', this.onVisibility)
	},
	// Run before component is removed
	beforeUnmount() {
		clearInterval(this.timer)
		document.removeEventListener('visibilitychange', this.onVisibility)
	},
	// Functions used by components
	methods: {
		onVisibility() {
			if (!document.hidden) this.poll()
		},
		async poll() {
			if (document.hidden || !this.chatId) return
			await this.load(true)
		},
		// Refresh displayed profiles separately from membership IDs
		async resolveNames(chat, membershipNotices = []) {
			const ids = []
			for (const m of chat.messages) {
				ids.push(m.user)
				for (const c of m.comments || []) ids.push(c.user)
			}
			for (const notice of membershipNotices) ids.push(notice.userId)
			await refreshMany(ids)
		},
		async load(silent = false) {
			const asked = this.chatId
			try {
				const chat = await getConversation(asked)
				// The route may have moved on while this was in flight
				if (this.chatId !== asked) return
				const membershipNotices =
					chat.chatType === 'group'
						? observeGroupMembers(session.userId, chat.id, chat.members)
						: []
				await this.resolveNames(chat, membershipNotices)
				if (this.chatId !== asked) return

				this.membershipNotices = [...membershipNotices]
				this.chat = chat
				this.errormsg = null
				this.scrollToBottomSoon()
			} catch (e) {
				if (this.chatId !== asked) return
				// A failed poll keeps the chat on screen; a failed open has nothing to keep
				if (!silent || !this.chat) this.errormsg = errorMessage(e)
			} finally {
				if (this.chatId === asked) this.loading = false
			}
		},

		// Saved notice authors are resolved together with current members and message senders
		username,

		// ----- scrolling -----
		onScroll() {
			const el = this.$refs.scroller
			if (!el) return
			// "At the bottom" with a little slack, so a pixel of overscroll does not unstick it
			this.stickToBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 60
		},
		scrollToBottomSoon(force = false) {
			if (!force && !this.stickToBottom) return
			// The DOM updates are buffered: the new height only exists after the next tick
			this.$nextTick(() => {
				const el = this.$refs.scroller
				if (el) el.scrollTop = el.scrollHeight
			})
		},

		// ----- replying -----
		startReply({ message }) {
			this.replyingTo = message
			this.$nextTick(() => this.$refs.composer && this.$refs.composer.focus())
		},
		cancelReply() {
			this.replyingTo = null
		},
		// Clicking a quote goes to the message it names, if the page still holds it
		jumpTo(messageId) {
			const el = this.$refs.scroller && this.$refs.scroller.querySelector(`[data-message-id="${messageId}"]`)
			if (!el) return
			el.scrollIntoView({ behavior: 'smooth', block: 'center' })
			// The flash is what tells the reader which message it landed on, in a wall of similar ones
			this.highlighted = messageId
			setTimeout(() => {
				if (this.highlighted === messageId) this.highlighted = null
			}, 1600)
		},

		// ----- writing -----
		// sendMessage, forwardMessage and commentMessage all answer with the whole message,
		// so the copy on screen is replaced from the reply instead of waiting for the next poll
		upsertMessage(message) {
			if (!this.chat) return
			const i = this.chat.messages.findIndex((m) => m.id === message.id)
			if (i >= 0) this.chat.messages.splice(i, 1, message)
			else this.chat.messages.unshift(message) // the list is newest first
		},
		async onSend({ text, file, replyTo }) {
			this.sending = true
			this.actionError = null
			try {
				const message = await sendMessage(this.chatId, { text, file, replyTo })
				this.upsertMessage(message)
				await resolveMany([message.user])
				this.$refs.composer.reset()
				this.replyingTo = null
				// Sending is always a reason to jump to the bottom, wherever the reader was
				this.stickToBottom = true
				this.scrollToBottomSoon(true)
			} catch (e) {
				this.actionError = errorMessage(e)
			} finally {
				this.sending = false
			}
		},
		async onDelete({ message }) {
			if (!window.confirm('Delete this message? It disappears for everyone in the chat.')) return
			this.actionError = null
			try {
				await deleteMessage(this.chatId, message.id)
				// 204 carries no body, so the row is dropped here and the next poll confirms it
				this.chat.messages = this.chat.messages.filter((m) => m.id !== message.id)
				// Answering a message that is gone would be refused by the server anyway
				if (this.replyingTo && this.replyingTo.id === message.id) this.replyingTo = null
			} catch (e) {
				this.actionError = errorMessage(e)
			}
		},
		async onReact({ message, emoji }) {
			this.actionError = null
			try {
				const updated = await commentMessage(this.chatId, message.id, emoji)
				this.upsertMessage(updated)
			} catch (e) {
				this.actionError = errorMessage(e)
			}
		},
		async onUnreact({ message }) {
			this.actionError = null
			try {
				await uncommentMessage(this.chatId, message.id)
				// 204 again: the caller's own reaction is the one row that went
				const i = this.chat.messages.findIndex((m) => m.id === message.id)
				if (i >= 0) {
					const m = this.chat.messages[i]
					this.chat.messages.splice(i, 1, {
						...m,
						comments: (m.comments || []).filter((c) => c.user !== session.userId),
					})
				}
			} catch (e) {
				this.actionError = errorMessage(e)
			}
		},

		// ----- forwarding -----
		openForward({ message }) {
			this.forwarding = message
		},
		async onForward(destination) {
			const message = this.forwarding
			this.actionError = null
			try {
				const copy = await forwardMessage(destination.id, message.id)
				this.forwarding = null
				if (destination.id === this.chatId) {
					// Forwarding into the chat that is open: the copy belongs on screen right away
					this.upsertMessage(copy)
					this.stickToBottom = true
					this.scrollToBottomSoon(true)
				} else {
					this.$router.push(`/chats/${destination.id}`)
				}
			} catch (e) {
				this.forwarding = null
				this.actionError = errorMessage(e)
			}
		},
	},
}
</script>

<template>
  <div class="conversation d-flex flex-column">
    <!-- header -->
    <div v-if="chat" class="chat-header d-flex align-items-center gap-2 border-bottom px-2 py-2">
      <RouterLink to="/" class="btn btn-sm btn-outline-secondary d-md-none" title="Back">&lsaquo;</RouterLink>
      <AuthPhoto :src="chat.photo" :alt="chat.name" :placeholder="chat.name" :size="42" />
      <div class="flex-grow-1 min-width-0">
        <div class="fw-semibold text-truncate">{{ chat.name }}</div>
        <div class="small text-body-secondary">
          {{ isGroup ? `${chat.members.length} members` : 'Private chat' }}
        </div>
      </div>
      <RouterLink
        v-if="isGroup"
        :to="`/groups/${chat.id}/info`"
        class="btn btn-sm btn-outline-secondary"
      >
        Group info
      </RouterLink>
    </div>

    <!-- messages -->
    <div ref="scroller" class="chat-scroller flex-grow-1" @scroll="onScroll">
      <LoadingSpinner :loading="loading">
        <ErrorMsg v-if="errormsg" :msg="errormsg" class="m-3" />

        <div v-else-if="chat && !timeline.length" class="text-center text-body-secondary p-5">
          No message yet. Write the first one.
        </div>

        <div v-else-if="chat" class="p-2">
          <template v-for="item in timeline" :key="item.key">
            <div v-if="item.type === 'day'" class="chat-notice">
              <span>{{ item.label }}</span>
            </div>

            <div v-else-if="item.type === 'left'" class="chat-notice">
              <span>{{ username(item.userId) }} left the chat</span>
            </div>

            <div v-else-if="item.type === 'joined'" class="chat-notice">
              <span>{{ username(item.userId) }} joined the chat</span>
            </div>

            <div
              v-else
              :data-message-id="item.message.id"
              :class="{ 'message-highlight': highlighted === item.message.id }"
            >
              <MessageBubble
                :message="item.message"
                :is-group="isGroup"
                :replied-to="messagesById.get(item.message.replyTo) || null"
                @react="onReact"
                @unreact="onUnreact"
                @forward="openForward"
                @delete="onDelete"
                @reply="startReply"
                @jump="jumpTo"
              />
            </div>
          </template>
        </div>
      </LoadingSpinner>
    </div>

    <ErrorMsg v-if="actionError" :msg="actionError" class="m-2 mb-0" />

    <MessageComposer
      v-if="chat"
      ref="composer"
      :sending="sending"
      :replying-to="replyingTo"
      @send="onSend"
      @cancel-reply="cancelReply"
    />

    <ForwardDialog
      v-if="forwarding"
      :message="forwarding"
      @close="forwarding = null"
      @forward="onForward"
    />
  </div>
</template>

<style scoped>
.conversation {
	height: 100%;
	min-height: 0;
}

.min-width-0 {
	min-width: 0;
}

.chat-header {
	background-color: #f8f9fa;
}

.chat-scroller {
	overflow-y: auto;
	min-height: 0;
	background-color: #efeae2;
}

/* Where a clicked quote lands: a short flash, so the reader sees which message it was sent to */
.message-highlight {
	animation: message-flash 1.6s ease-out;
	border-radius: 0.75rem;
}

@keyframes message-flash {
	0%,
	40% {
		background-color: rgba(13, 110, 253, 0.18);
	}
	100% {
		background-color: transparent;
	}
}

/* The day separators and the departure notices are the same thing on screen:
	a centred caption between two bubbles, belonging to the chat rather than to anybody in it */
.chat-notice {
	text-align: center;
	margin: 0.75rem 0;
}

.chat-notice span {
	background-color: #ffffff;
	border-radius: 1rem;
	padding: 0.15rem 0.75rem;
	font-size: 0.75rem;
	color: #54656f;
	box-shadow: 0 1px 1px rgba(0, 0, 0, 0.08);
}
</style>
