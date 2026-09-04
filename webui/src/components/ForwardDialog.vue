<script>
import { getMyConversations, errorMessage } from '../services/api.js'
import AuthPhoto from './AuthPhoto.vue'

// Picking where a message goes.
//
// forwardMessage names the source message in the body and the destination chat in the URL,
// and both kinds of chat take it, including the chat the message is already in.
// The caller must belong to both, and every chat in this list is one it belongs to by definition.
export default {
	// Reusable components
	components: { AuthPhoto },
	// Values received from a parent
	props: {
		// The message being forwarded, only to show what is being sent
		message: { type: Object, required: true },
	},
	// Update parents
	emits: ['close', 'forward'],
	// Reactive state for each component instance
	data() {
		return {
			chats: [],
			loading: true,
			errormsg: null,
			sendingTo: null,
		}
	},
	// Values derived from reactive values
	computed: {
		preview() {
			const c = this.message.content || {}
			if (c.text && c.photo) return `📷 ${c.text}`
			if (c.photo) return '📷 Photo'
			return c.text || ''
		},
	},
	// Run after component is shown
	async mounted() {
		try {
			this.chats = await getMyConversations()
		} catch (e) {
			this.errormsg = errorMessage(e)
		} finally {
			this.loading = false
		}
	},
	// Functions used by components
	methods: {
		// The parent owns the call and closes this dialog:
		// here the row is only marked as the one being sent to,
		// so a slow forward does not look like a click that did nothing
		choose(chat) {
			if (this.sendingTo) return
			this.sendingTo = chat.id
			this.$emit('forward', chat)
		},
	},
}
</script>

<template>
  <div class="modal-backdrop-custom" @click.self="$emit('close')">
    <div class="card shadow forward-dialog">
      <div class="card-header d-flex justify-content-between align-items-center">
        <strong>Forward to…</strong>
        <button type="button" class="btn-close" aria-label="Close" @click="$emit('close')" />
      </div>

      <div class="card-body py-2 border-bottom">
        <div class="text-body-secondary small">Message</div>
        <div class="text-truncate">{{ preview }}</div>
      </div>

      <div class="forward-list">
        <ErrorMsg v-if="errormsg" :msg="errormsg" class="m-3" />
        <div v-else-if="loading" class="p-3 text-body-secondary">Loading your conversations…</div>
        <p v-else-if="!chats.length" class="p-3 text-body-secondary mb-0">
          You have no other conversation to forward this to.
        </p>
        <ul v-else class="list-group list-group-flush">
          <li
            v-for="chat in chats"
            :key="chat.id"
            class="list-group-item list-group-item-action d-flex align-items-center gap-2"
            role="button"
            @click="choose(chat)"
          >
            <AuthPhoto :src="chat.photo" :alt="chat.name" :placeholder="chat.name" :size="36" />
            <span class="flex-grow-1 text-truncate">
              {{ chat.name }}
              <span class="badge text-bg-light ms-1">{{ chat.chatType }}</span>
            </span>
            <span v-if="sendingTo === chat.id" class="spinner-border spinner-border-sm" />
          </li>
        </ul>
      </div>
    </div>
  </div>
</template>

<style scoped>
.modal-backdrop-custom {
	position: fixed;
	inset: 0;
	background-color: rgba(0, 0, 0, 0.45);
	display: flex;
	align-items: center;
	justify-content: center;
	z-index: 1080;
	padding: 1rem;
}

.forward-dialog {
	width: 100%;
	max-width: 26rem;
}

.forward-list {
	max-height: 60vh;
	overflow-y: auto;
}
</style>
