<script>
import { getMyConversations, errorMessage } from '../services/api.js'
import { POLL_MS } from '../services/axios.js'
import { session } from '../services/session.js'
import { listTime, snippetText } from '../services/format.js'
import AuthPhoto from './AuthPhoto.vue'

// The homepage list of the project spec: every chat the caller belongs to,
// most recently active first, each with the name, the photo, the time of the last message and a preview of it.
//
// The server sorts it (a chat nobody has written in has no date and lands at the bottom),
// so nothing is reordered here. It answers 404 for an empty list, which api.js already reads as [].
//
// There is no push channel in this API, so the list is polled.
// The interval pauses while the tab is hidden: a background tab that nobody is reading does not need to stay current.
export default {
	// Reusable components
	components: { AuthPhoto },
	// Update parents
	emits: ['loaded'],
	// Reactive state for each component instance
	data() {
		return {
			chats: [],
			// Only the first load blanks the list:
			// a poll that fails or is slow must not make the conversations flicker away under the reader
			loading: true,
			errormsg: null,
			timer: null,
		}
	},
	// Values derived from reactive values
	computed: {
		activeChatId() {
			return this.$route.params.chatId || null
		},
	},
	// Run after component is shown
	mounted() {
		this.refresh()
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
		listTime,
		snippetText,
		onVisibility() {
			// Coming back to the tab should show the current list at once, not after a whole interval
			if (!document.hidden) this.poll()
		},
		async poll() {
			if (document.hidden) return
			await this.refresh(true)
		},
		async refresh(silent = false) {
			try {
				this.chats = await getMyConversations()
				this.errormsg = null
				this.$emit('loaded', this.chats)
			} catch (e) {
				// A failed poll keeps what is on screen: the previous answer is still the best guess
				if (!silent) this.errormsg = errorMessage(e)
			} finally {
				this.loading = false
			}
		},
		// The checkmarks belong to the sender, so the preview shows them only when the last message
		// is the caller's own
		ticks(chat) {
			if (!chat.snippet || chat.snippet.user !== session.userId) return null
			return chat.snippet.state === 'read' ? '✓✓' : '✓'
		},
	},
}
</script>

<template>
  <div class="conversation-list d-flex flex-column h-100">
    <ErrorMsg v-if="errormsg" :msg="errormsg" class="m-2" />

    <div v-if="loading" class="p-3 text-body-secondary small">Loading conversations…</div>

    <p v-else-if="!chats.length" class="p-3 text-body-secondary small mb-0">
      No conversation yet. Start a new chat or create a group.
    </p>

    <ul v-else class="list-group list-group-flush chat-rows">
      <li v-for="chat in chats" :key="chat.id" class="list-group-item p-0 border-0">
        <RouterLink
          :to="`/chats/${chat.id}`"
          :class="[
            'chat-row d-flex align-items-center gap-2 p-2 text-decoration-none',
            chat.id === activeChatId ? 'chat-row-active' : '',
          ]"
        >
          <AuthPhoto :src="chat.photo" :alt="chat.name" :placeholder="chat.name" :size="44" />

          <span class="flex-grow-1 min-width-0">
            <span class="d-flex justify-content-between align-items-baseline gap-2">
              <span class="chat-name text-truncate">{{ chat.name }}</span>
              <span v-if="chat.snippet" class="chat-time flex-shrink-0">
                {{ listTime(chat.snippet.date) }}
              </span>
            </span>
            <span class="d-flex align-items-baseline gap-1">
              <span v-if="ticks(chat)" class="chat-ticks flex-shrink-0">{{ ticks(chat) }}</span>
              <span class="chat-snippet text-truncate">
                {{ chat.snippet ? snippetText(chat.snippet) : 'No message yet' }}
              </span>
              <span v-if="chat.chatType === 'group'" class="badge text-bg-light flex-shrink-0 ms-auto">
                group
              </span>
            </span>
          </span>
        </RouterLink>
      </li>
    </ul>
  </div>
</template>

<style scoped>
/* The rows are the part that scrolls: min-height 0 is what lets a flex item shrink below its
   content instead of pushing the column past the bottom of the window */
.chat-rows {
	flex: 1 1 auto;
	min-height: 0;
	overflow-y: auto;
}

.min-width-0 {
	min-width: 0;
}

.chat-row {
	color: inherit;
	border-bottom: 1px solid #e9ecef;
}

.chat-row:hover {
	background-color: #f1f3f5;
}

.chat-row-active {
	background-color: #e7f1ff;
}

.chat-name {
	font-weight: 600;
	display: block;
}

.chat-time,
.chat-snippet {
	font-size: 0.78rem;
	color: #6c757d;
	display: block;
}

.chat-ticks {
	font-size: 0.78rem;
	color: #53bdeb;
	font-weight: 700;
	letter-spacing: -0.15em;
}
</style>
