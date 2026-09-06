<script>
import { createGroup, errorMessage } from '../services/api.js'
import { remember } from '../services/users.js'
import { session } from '../services/session.js'
import { countChars } from '../services/format.js'
import UserSearch from '../components/UserSearch.vue'
import AuthPhoto from '../components/AuthPhoto.vue'

// Creating a group.
//
// createGroup takes a name, the other members and, optionally, a photo.
// Without one the group starts with a default photo and keeps it until somebody replaces it,
// so the picker below offers a choice and never a requirement.
// The creator must not be in the list, because the server adds it.
// A group holds 100 members, so 99 can be picked here.
const MAX_OTHERS = 99

export default {
	// Reusable components
	components: { UserSearch, AuthPhoto },
	// Reactive state for each component instance
	data() {
		return {
			name: '',
			picked: [],
			file: null,
			filePreview: null,
			creating: false,
			errormsg: null,
			accept: 'image/png,image/jpeg,image/gif,image/webp',
			maxOthers: MAX_OTHERS,
			// The id every new user starts from (schemas.DefaultPhotoId), shown as the group's photo
			// until one is picked — the same thing the server will store
			defaultPhoto: '/photos/00000000-0000-4000-8000-000000000000',
		}
	},
	// Values derived from reactive values
	computed: {
		// Counted the way the server counts it
		nameCount() {
			return countChars(this.name)
		},
		nameValid() {
			return this.nameCount >= 1 && this.nameCount <= 100
		},
		// The caller is never offered: it is a member by definition and listing it is a 400
		excluded() {
			return [session.userId, ...this.picked.map((u) => u.id)]
		},
		canCreate() {
			return (
				this.nameValid &&
				this.picked.length >= 1 && this.picked.length <= MAX_OTHERS &&
				!this.creating
			)
		},
	},
	// Run before component is removed
	beforeUnmount() {
		this.revokePreview()
	},
	// Functions used by components
	methods: {
		revokePreview() {
			if (this.filePreview) {
				URL.revokeObjectURL(this.filePreview)
				this.filePreview = null
			}
		},
		pickPhoto(event) {
			const file = event.target.files && event.target.files[0]
			event.target.value = ''
			if (!file) return
			this.revokePreview()
			this.file = file
			this.filePreview = URL.createObjectURL(file)
		},
		addMember(user) {
			if (this.picked.some((u) => u.id === user.id)) return
			if (this.picked.length >= MAX_OTHERS) return
			remember(user)
			this.picked.push(user)
		},
		clearPhoto() {
			this.revokePreview()
			this.file = null
		},
		removeMember(userId) {
			this.picked = this.picked.filter((u) => u.id !== userId)
		},
		async create() {
			if (!this.canCreate) return
			this.creating = true
			this.errormsg = null
			try {
				const group = await createGroup({
					name: this.name.trim(),
					members: this.picked.map((u) => u.id),
					file: this.file,
				})
				this.$router.push(`/chats/${group.id}`)
			} catch (e) {
				this.errormsg = errorMessage(e)
			} finally {
				this.creating = false
			}
		},
	},
}
</script>

<template>
  <div class="py-3 px-3">
    <div class="d-flex align-items-center justify-content-between border-bottom pb-2 mb-3">
      <h1 class="h4 mb-0">New group</h1>
      <RouterLink to="/" class="btn btn-sm btn-outline-secondary">Cancel</RouterLink>
    </div>

    <div class="new-group-body">
      <ErrorMsg v-if="errormsg" :msg="errormsg" />

      <section class="mb-4">
        <label for="group-name" class="form-label fw-semibold">Name</label>
        <input
          id="group-name"
          v-model="name"
          type="text"
          class="form-control"
          placeholder="e.g. Study Group"
          :disabled="creating"
        >
        <div class="form-text">
          1 to 100 characters. <span v-if="nameCount">{{ nameCount }} / 100</span>
        </div>
      </section>

      <section class="mb-4">
        <div class="fw-semibold mb-1">
          Photo <span class="text-body-secondary fw-normal">(optional)</span>
        </div>
        <div class="d-flex align-items-center gap-3">
          <img v-if="filePreview" :src="filePreview" alt="Group photo" class="group-photo-preview">
          <AuthPhoto v-else :src="defaultPhoto" alt="Default group photo" :size="80" />
          <div>
            <label class="btn btn-outline-primary mb-1">
              {{ file ? 'Change photo' : 'Choose a photo' }}
              <input type="file" class="d-none" :accept="accept" :disabled="creating" @change="pickPhoto">
            </label>
            <button
              v-if="file"
              type="button"
              class="btn btn-link btn-sm mb-1 ms-1"
              @click="clearPhoto"
            >
              Use the default
            </button>
            <div class="form-text">
              Left alone, the group starts from the default photo and anyone can change it later.
              PNG, JPEG, GIF or WEBP, up to 30 MB.
            </div>
          </div>
        </div>
      </section>

      <section class="mb-4">
        <div class="fw-semibold mb-1">
          Members
          <span class="text-body-secondary fw-normal">
            ({{ picked.length }} of {{ maxOthers }}, plus you)
          </span>
        </div>

        <div v-if="picked.length" class="d-flex flex-wrap gap-2 mb-2">
          <span v-for="user in picked" :key="user.id" class="badge text-bg-light border d-inline-flex align-items-center gap-1 p-1">
            <AuthPhoto :src="user.photo" :alt="user.username" :placeholder="user.username" :size="22" />
            <span>{{ user.username }}</span>
            <button
              type="button"
              class="btn-close btn-close-sm ms-1"
              :aria-label="`Remove ${user.username}`"
              @click="removeMember(user.id)"
            />
          </span>
        </div>

        <UserSearch
          :excluded="excluded"
          excluded-label="picked"
          placeholder="Add someone by username"
          @pick="addMember"
        />

        <p v-if="!picked.length" class="form-text mb-0">Pick at least one other member.</p>
      </section>

      <button type="button" class="btn btn-primary" :disabled="!canCreate" @click="create">
        {{ creating ? 'Creating…' : 'Create group' }}
      </button>
    </div>
  </div>
</template>

<style scoped>
.new-group-body {
	max-width: 34rem;
}

.group-photo-preview {
	width: 80px;
	height: 80px;
	border-radius: 50%;
	object-fit: cover;
	flex-shrink: 0;
}
</style>
