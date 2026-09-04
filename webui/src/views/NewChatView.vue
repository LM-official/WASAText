<script>
import { createPrivateChat, errorMessage } from '../services/api.js'
import { remember } from '../services/users.js'
import UserSearch from '../components/UserSearch.vue'

// Starting a conversation with any other user of WASAText.
//
// createPrivateChat answers 201 for a new chat and 200 for the one that already existed,
// with the same id either way: the pair is one row,
// so picking somebody already talked to reopens that chat instead of making a second one.
// Either way the view goes to the chat it names.
export default {
	// Reusable components
	components: { UserSearch },
	// Reactive state for each component instance
	data() {
		return {
			opening: false,
			errormsg: null,
		}
	},
	// Functions used by components
	methods: {
		async pick(user) {
			if (this.opening) return
			this.opening = true
			this.errormsg = null
			try {
				remember(user)
				const chatId = await createPrivateChat(user.id)
				this.$router.push(`/chats/${chatId}`)
			} catch (e) {
				this.errormsg = errorMessage(e)
			} finally {
				this.opening = false
			}
		},
	},
}
</script>

<template>
  <div class="py-3 px-3">
    <div class="d-flex align-items-center justify-content-between border-bottom pb-2 mb-3">
      <h1 class="h4 mb-0">New chat</h1>
      <RouterLink to="/" class="btn btn-sm btn-outline-secondary">Cancel</RouterLink>
    </div>

    <div class="new-chat-body">
      <p class="text-body-secondary">
        Search any WASAText user by username.
        If you already have a conversation with them, it opens again rather than starting a second one.
      </p>

      <ErrorMsg v-if="errormsg" :msg="errormsg" />

      <UserSearch autofocus placeholder="Who do you want to write to?" @pick="pick" />

      <div v-if="opening" class="text-body-secondary small mt-2">Opening the conversation…</div>
    </div>
  </div>
</template>

<style scoped>
.new-chat-body {
	max-width: 32rem;
}
</style>
