<script>
import { getGroup, setGroupName, setGroupPhoto, addToGroup, leaveGroup, errorMessage } from '../services/api.js'
import { session } from '../services/session.js'
import { profile, refreshMany, remember } from '../services/users.js'
import { observeGroupMembers } from '../services/membership-notices.js'
import { countChars } from '../services/format.js'
import AuthPhoto from '../components/AuthPhoto.vue'
import UserSearch from '../components/UserSearch.vue'

// Group info refreshes on entry and return to the tab/window, never on an interval.
// Its metadata endpoint does not read messages or advance read receipts
export default {
	// Reusable components
	components: { AuthPhoto, UserSearch },
	// Reactive state for each component instance
	data() {
		return {
			chat: null,
			loading: true,
			errormsg: null,
			name: '',
			savingName: false,
			nameError: null,
			savingPhoto: false,
			photoError: null,
			addingError: null,
			adding: false,
			leaving: false,
			ok: null,
			accept: 'image/png,image/jpeg,image/gif,image/webp',
			returnTimer: null,
			refreshPromise: null,
			refreshPending: false,
			viewVersion: 0,
			disposed: false,
		}
	},
	// Values derived from reactive values
	computed: {
		session() { return session },
		chatId() { return this.$route.params.groupId },
		nameCount() { return countChars(this.name) },
		nameValid() { return this.nameCount >= 1 && this.nameCount <= 100 },
		nameUnchanged() { return !!this.chat && this.name.trim() === this.chat.name },
		busy() { return this.savingName || this.savingPhoto || this.adding || this.leaving },
		members() {
			return this.memberIds.map(profile).sort((a, b) => {
				if (a.id === b.id) return 0
				if (a.id === session.userId) return -1
				if (b.id === session.userId) return 1
				return a.username.localeCompare(b.username) || a.id.localeCompare(b.id)
			})
		},
		memberIds() { return this.chat ? this.chat.members : [] },
		full() { return this.memberIds.length >= 100 },
	},
	// Run when a reactive value changes
	watch: {
		chatId: {
			immediate: true,
			handler() {
				this.viewVersion++
				clearTimeout(this.returnTimer)
				this.refreshPromise = null
				this.refreshPending = false
				this.chat = null
				this.name = ''
				this.ok = null
				this.nameError = this.photoError = this.addingError = null
				this.savingName = this.savingPhoto = this.adding = this.leaving = false
				this.load()
			},
		},
		'session.userId'() {
			this.viewVersion++
			this.chat = null
			this.refreshPromise = null
			if (session.userId) this.load()
		},
	},
	// Run after component is shown
	mounted() {
		window.addEventListener('focus', this.onReturn)
		document.addEventListener('visibilitychange', this.onReturn)
	},
	// Run before component is removed
	beforeUnmount() {
		this.disposed = true
		this.viewVersion++
		clearTimeout(this.returnTimer)
		window.removeEventListener('focus', this.onReturn)
		document.removeEventListener('visibilitychange', this.onReturn)
	},
	// Functions used by components
	methods: {
		current(id, owner, version) {
			return !this.disposed && this.chatId === id && session.userId === owner && this.viewVersion === version
		},
		onReturn() {
			if (document.hidden) return
			clearTimeout(this.returnTimer)
			// Focus and visibility often fire together; this is a one-shot debounce
			this.returnTimer = setTimeout(() => {
				if (!document.hidden) this.load(true)
			}, 150)
		},
		load(silent = false) {
			if (this.disposed || !session.userId) return Promise.resolve()
			if (this.busy) {
				this.refreshPending = true
				return this.refreshPromise || Promise.resolve()
			}
			if (this.refreshPromise) return this.refreshPromise
			this.refreshPending = false
			if (!silent) this.loading = true
			this.errormsg = null
			const id = this.chatId
			const owner = session.userId
			const version = this.viewVersion
			this.refreshPromise = this.readGroup(id, owner, version).finally(() => {
				if (!this.current(id, owner, version)) return
				this.refreshPromise = null
				this.loading = false
			})
			return this.refreshPromise
		},
		async readGroup(id, owner, version) {
			try {
				const group = await getGroup(id)
				if (!this.current(id, owner, version)) return
				// Compare with the old server value, including edits made during the request
				const pristine = !this.chat || this.name.trim() === this.chat.name
				this.chat = group
				if (pristine) this.name = group.name
				observeGroupMembers(owner, group.id, group.members)
				const refreshed = await refreshMany(group.members)
				if (this.current(id, owner, version) && !refreshed) {
					this.errormsg = 'Some member profiles could not be refreshed. Return to this window to retry.'
				}
			} catch (e) {
				if (!this.current(id, owner, version)) return
				if ([403, 404].includes(e.response && e.response.status)) this.chat = null
				this.errormsg = errorMessage(e)
			}
		},
		// Serialize writes with the current refresh so an earlier read cannot undo a write
		async mutate(flag, errorField, operation, apply) {
			if (this.busy || !this.chat) return
			const id = this.chatId
			const owner = session.userId
			const version = this.viewVersion
			this[flag] = true
			this[errorField] = null
			this.ok = null
			let changed = false
			try {
				await this.refreshPromise
				if (!this.current(id, owner, version) || !this.chat) return
				const result = await operation(id)
				if (!this.current(id, owner, version)) return
				apply(result)
				changed = true
			} catch (e) {
				if (!this.current(id, owner, version)) return
				if ([403, 404].includes(e.response && e.response.status)) {
					// A failed add may also mean the selected user disappeared.
					// Re-read metadata before deciding whether group access was lost
					this.refreshPending = true
				}
				this[errorField] = errorMessage(e)
			} finally {
				if (this.current(id, owner, version)) {
					this[flag] = false
					if (this.chat && (changed || this.refreshPending)) await this.load(true)
				}
			}
		},
		async saveName() {
			if (!this.nameValid) return
			const submitted = this.name.trim()
			await this.mutate('savingName', 'nameError', (id) => setGroupName(id, submitted), (group) => {
				if (this.name.trim() === submitted) this.name = group.name
				this.applyGroup(group)
				this.ok = 'Group name updated.'
			})
		},
		async pickPhoto(event) {
			const file = event.target.files && event.target.files[0]
			event.target.value = ''
			if (!file) return
			await this.mutate('savingPhoto', 'photoError', (id) => setGroupPhoto(id, file), (group) => {
				this.applyGroup(group)
				this.ok = 'Group photo updated.'
			})
		},
		async add(user) {
			await this.mutate('adding', 'addingError', (id) => addToGroup(id, [user.id]), (group) => {
				remember(user)
				this.applyGroup(group)
				observeGroupMembers(session.userId, group.id, group.members)
				this.ok = `${profile(user.id).username} was added to the group.`
			})
		},
		applyGroup(group) {
			const pristine = this.name.trim() === this.chat.name
			Object.assign(this.chat, group)
			if (pristine) this.name = group.name
		},
		async leave() {
			if (this.busy || !this.chat) return
			const question = this.memberIds.length === 1
				? 'You are the last member: leaving deletes this group and all of its messages. Continue?'
				: 'Leave this group? You will stop seeing it and will need to be added again.'
			if (!window.confirm(question)) return
			await this.mutate('leaving', 'errormsg', leaveGroup, () => {
				this.chat = null
				this.$router.push('/')
			})
		},
	},
}
</script>

<template>
  <div class="py-3 px-3">
    <div class="d-flex align-items-center justify-content-between border-bottom pb-2 mb-3">
      <h1 class="h4 mb-0">Group info</h1>
      <RouterLink :to="`/chats/${chatId}`" class="btn btn-sm btn-outline-secondary">
        Back to chat
      </RouterLink>
    </div>

    <LoadingSpinner :loading="loading">
      <ErrorMsg v-if="errormsg" :msg="errormsg" />

      <div v-if="chat" class="group-info-body">
        <div v-if="ok" class="alert alert-success" role="alert">{{ ok }}</div>

        <section class="mb-4">
          <h2 class="h6 text-body-secondary text-uppercase">Photo</h2>
          <div class="d-flex align-items-center gap-3">
            <AuthPhoto :src="chat.photo" :alt="chat.name" :placeholder="chat.name" :size="96" />
            <div>
              <label class="btn btn-outline-primary mb-1">
                {{ savingPhoto ? 'Uploading…' : 'Change photo' }}
                <input type="file" class="d-none" :accept="accept" :disabled="savingPhoto" @change="pickPhoto">
              </label>
              <div class="form-text">Any member may change it.</div>
            </div>
          </div>
          <ErrorMsg v-if="photoError" :msg="photoError" class="mt-2" />
        </section>

        <section class="mb-4">
          <h2 class="h6 text-body-secondary text-uppercase">Name</h2>
          <form class="row g-2 align-items-start" @submit.prevent="saveName">
            <div class="col-sm-8">
              <input v-model="name" type="text" class="form-control" :disabled="savingName">
              <div class="form-text">1 to 100 characters. {{ nameCount }} / 100</div>
            </div>
            <div class="col-sm-4">
              <button
                type="submit"
                class="btn btn-primary"
                :disabled="!nameValid || nameUnchanged || savingName"
              >
                {{ savingName ? 'Saving…' : 'Rename' }}
              </button>
            </div>
          </form>
          <ErrorMsg v-if="nameError" :msg="nameError" class="mt-2" />
        </section>

        <section class="mb-4">
          <h2 class="h6 text-body-secondary text-uppercase">
            Members <span class="text-body-secondary">({{ chat.members.length }} of 100)</span>
          </h2>
          <ul class="list-group mb-3">
            <li
              v-for="member in members"
              :key="member.id"
              class="list-group-item d-flex align-items-center gap-2"
            >
              <AuthPhoto :src="member.photo" :alt="member.username" :placeholder="member.username" :size="32" />
              <span class="flex-grow-1 text-truncate">{{ member.username }}</span>
              <span v-if="member.id === session.userId" class="badge text-bg-primary">you</span>
            </li>
          </ul>

          <div v-if="full" class="alert alert-warning py-2 mb-0">
            This group is full: 100 members is the limit.
          </div>
          <template v-else>
            <div class="fw-semibold mb-1">Add a member</div>
            <ErrorMsg v-if="addingError" :msg="addingError" />
            <UserSearch
              :excluded="memberIds"
              excluded-label="member"
              placeholder="Add someone by username"
              @pick="add"
            />
          </template>
        </section>

        <section class="border-top pt-3">
          <button type="button" class="btn btn-outline-danger" :disabled="leaving" @click="leave">
            {{ leaving ? 'Leaving…' : 'Leave group' }}
          </button>
          <p class="form-text mb-0">
            Only you can remove yourself. If you are the last member, the group and its messages are deleted.
          </p>
        </section>
      </div>
    </LoadingSpinner>
  </div>
</template>

<style scoped>
.group-info-body {
	max-width: 34rem;
}
</style>
