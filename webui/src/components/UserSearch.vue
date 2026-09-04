<script>
import { getUsers, errorMessage } from '../services/api.js'
import { rememberAll } from '../services/users.js'
import { session } from '../services/session.js'
import AuthPhoto from './AuthPhoto.vue'

// The username search box, shared by "new chat", "new group" and "add members".
//
// GET /users takes a username prefix and answers 404 when it matches nobody,
// which is an empty result and not a failure.
// The caller is in its own results (nothing filters it out server side)
// and opening a private chat with yourself is a 400, so it is hidden here.
export default {
	// Reusable components
	components: { AuthPhoto },
	// Values received from a parent
	props: {
		placeholder: { type: String, default: 'Search by username' },
		// Ids already picked, or already members: shown as disabled rather than hidden, so it is
		// clear the user was found and is simply already in
		excluded: { type: Array, default: () => [] },
		excludedLabel: { type: String, default: 'already added' },
		autofocus: { type: Boolean, default: false },
	},
	// Update parents
	emits: ['pick'],
	// Reactive state for each component instance
	data() {
		return {
			query: '',
			results: [],
			loading: false,
			errormsg: null,
			searched: false,
			timer: null,
			// Only the answer to the latest keystroke is allowed to reach the list
			lastRequest: 0,
		}
	},
	// Values derived from reactive values
	computed: {
		// The server refuses anything outside this, so an invalid prefix is never sent.
		// Same regex used for username validation
		queryIsValid() {
			return /^[a-zA-Z0-9_.-]{1,30}$/.test(this.query)
		},
	},
	// Run when a reactive value changes
	watch: {
		query() {
			// Debounced: a search runs on a pause in the typing and not on every character
			clearTimeout(this.timer)
			this.errormsg = null
			if (!this.query) {
				this.results = []
				this.searched = false
				return
			}
			this.timer = setTimeout(this.search, 300)
		},
	},
	// Run after component is shown
	mounted() {
		if (this.autofocus) this.$refs.input.focus()
	},
	// Run before component is removed
	beforeUnmount() {
		clearTimeout(this.timer)
	},
	// Functions used by components
	methods: {
		isExcluded(userId) {
			return this.excluded.includes(userId)
		},
		async search() {
			if (!this.queryIsValid) {
				this.results = []
				this.searched = true
				return
			}

			const request = ++this.lastRequest
			this.loading = true
			this.errormsg = null
			try {
				const users = await getUsers(this.query)
				if (request !== this.lastRequest) return
				// The results are users the app will want by id later on
				rememberAll(users)
				// Never offer the caller: a private chat with yourself is refused by the server
				this.results = users.filter((u) => u.id !== session.userId)
				this.searched = true
			} catch (e) {
				if (request !== this.lastRequest) return
				this.errormsg = errorMessage(e)
			} finally {
				if (request === this.lastRequest) this.loading = false
			}
		},
		pick(user) {
			if (this.isExcluded(user.id)) return
			this.$emit('pick', user)
		},
	},
}
</script>

<template>
  <div>
    <div class="input-group mb-2">
      <span class="input-group-text">@</span>
      <input
        ref="input"
        v-model.trim="query"
        type="search"
        class="form-control"
        :placeholder="placeholder"
        maxlength="30"
        @keydown.enter.prevent="search"
      >
    </div>

    <p v-if="query && !queryIsValid" class="form-text text-danger mb-2">
      A username is 1 to 30 characters, only letters, digits and <code>_ . -</code>
    </p>

    <ErrorMsg v-if="errormsg" :msg="errormsg" />

    <div v-if="loading" class="text-body-secondary small py-2">Searching…</div>

    <ul v-else-if="results.length" class="list-group user-search-results">
      <li
        v-for="user in results"
        :key="user.id"
        :class="[
          'list-group-item d-flex align-items-center gap-2',
          isExcluded(user.id) ? 'disabled text-body-secondary' : 'list-group-item-action',
        ]"
        role="button"
        @click="pick(user)"
      >
        <AuthPhoto :src="user.photo" :alt="user.username" :placeholder="user.username" :size="32" />
        <span class="flex-grow-1 text-truncate">{{ user.username }}</span>
        <span v-if="isExcluded(user.id)" class="badge text-bg-secondary">{{ excludedLabel }}</span>
      </li>
    </ul>

    <p v-else-if="searched && queryIsValid && !loading" class="text-body-secondary small py-2 mb-0">
      No user matches “{{ query }}”.
    </p>
  </div>
</template>

<style scoped>
.user-search-results {
	max-height: 18rem;
	overflow-y: auto;
}
</style>
