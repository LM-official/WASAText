<script>
import {
	getConversation,
	setGroupName,
	setGroupPhoto,
	addToGroup,
	leaveGroup,
	errorMessage,
} from '../services/api.js'
import { session } from '../services/session.js'
import { rememberAll, remember } from '../services/users.js'
import { observeGroupMembers } from '../services/membership-notices.js'
import { countChars } from '../services/format.js'
import AuthPhoto from '../components/AuthPhoto.vue'
import UserSearch from '../components/UserSearch.vue'

// Everything a group can be changed into.
//
// A group has no owner: every member may rename it, replace its photo and add anybody,
// and nobody can remove anybody but themselves, leaveGroup is the only way out and it only ever removes the caller.
// The last member to leave takes the group with it.
//
// The members arrive with their name and photo, so the page draws them without asking who they are,
// and every reply that changes the group carries the list again.
// What it does not do is re-read the conversation on a timer to catch a rename made elsewhere:
// that read marks the chat read behind the reader's back, which is what the checkmarks of everybody who wrote in it are made of.
// A name here is the one the group had when the page was opened
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
		}
	},
	// Values derived from reactive values
	computed: {
		session() {
			return session
		},
		chatId() {
			return this.$route.params.groupId
		},
		nameCount() {
			return countChars(this.name)
		},
		nameValid() {
			return this.nameCount >= 1 && this.nameCount <= 100
		},
		nameUnchanged() {
			// && gives back its left side when that is falsy: without !! this is null while loading
			return !!this.chat && this.name.trim() === this.chat.name
		},
		// The caller first, then everybody else by name
		members() {
			if (!this.chat) return []
			return [...this.chat.members].sort((a, b) => {
				if (a.id === session.userId) return -1
				if (b.id === session.userId) return 1
				return a.username.localeCompare(b.username)
			})
		},
		// observeGroupMembers diffs and saves ids, and the search excludes by id
		memberIds() {
			return this.chat ? this.chat.members.map((m) => m.id) : []
		},
		full() {
			// !! answers false before the group is read, where the && alone would say null
			return !!this.chat && this.chat.members.length >= 100
		},
	},
	// Run when a reactive value changes
	watch: {
		chatId: {
			immediate: true,
			handler() {
				this.load()
			},
		},
	},
	// Functions used by components
	methods: {
		async load() {
			this.loading = true
			this.errormsg = null
			try {
				const chat = await getConversation(this.chatId)
				if (chat.chatType !== 'group') {
					// Only a group has a name, a photo and a membership of its own to change
					this.$router.replace(`/chats/${this.chatId}`)
					return
				}
				rememberAll(chat.members)
				observeGroupMembers(session.userId, chat.id, chat.members.map((m) => m.id))
				this.chat = chat
				this.name = chat.name
			} catch (e) {
				this.errormsg = errorMessage(e)
			} finally {
				this.loading = false
			}
		},
		async saveName() {
			if (!this.nameValid || this.savingName) return
			this.savingName = true
			this.nameError = null
			this.ok = null
			try {
				// The reply is the group without its members, so only the base is refreshed from it
				const group = await setGroupName(this.chatId, this.name.trim())
				this.chat.name = group.name
				this.chat.photo = group.photo
				this.ok = 'Group name updated.'
			} catch (e) {
				this.nameError = errorMessage(e)
			} finally {
				this.savingName = false
			}
		},
		async pickPhoto(event) {
			const file = event.target.files && event.target.files[0]
			event.target.value = ''
			if (!file) return

			this.savingPhoto = true
			this.photoError = null
			this.ok = null
			try {
				const group = await setGroupPhoto(this.chatId, file)
				this.chat.name = group.name
				this.chat.photo = group.photo
				this.ok = 'Group photo updated.'
			} catch (e) {
				this.photoError = errorMessage(e)
			} finally {
				this.savingPhoto = false
			}
		},
		async add(user) {
			if (this.adding) return
			this.adding = true
			this.addingError = null
			this.ok = null
			try {
				remember(user)
				// The reply carries the whole member list after the write, so it is taken as it comes
				const group = await addToGroup(this.chatId, [user.id])
				this.chat.name = group.name
				this.chat.photo = group.photo
				this.chat.members = group.members
				rememberAll(group.members)
				observeGroupMembers(session.userId, group.id, group.members.map((m) => m.id))
				this.ok = `${user.username} was added to the group.`
			} catch (e) {
				this.addingError = errorMessage(e)
			} finally {
				this.adding = false
			}
		},
		async leave() {
			const last = this.chat.members.length === 1
			const question = last
				? 'You are the last member: leaving deletes this group and all of its messages. Continue?'
				: 'Leave this group? You will stop seeing it and will need to be added again.'
			if (!window.confirm(question)) return

			this.leaving = true
			this.errormsg = null
			try {
				await leaveGroup(this.chatId)
				this.$router.push('/')
			} catch (e) {
				this.errormsg = errorMessage(e)
			} finally {
				this.leaving = false
			}
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
