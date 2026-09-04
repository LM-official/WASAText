<script>
import { setMyUserName, setMyPhoto, errorMessage } from '../services/api.js'
import { session, setUser, logout } from '../services/session.js'
import { remember, clear as clearUsers } from '../services/users.js'
import { revokeAll } from '../services/photos.js'
import AuthPhoto from '../components/AuthPhoto.vue'

// The user's own name and photo.
//
// setMyUserName refuses a name another user already holds, with `400 username already taken`;
// that message is shown as it comes, since it is the one thing the user can act on.
// Both endpoints answer with the whole User, so the session is refreshed from the reply and never guessed.
export default {
	// Reusable components
	components: { AuthPhoto },
	// Reactive state for each component instance
	data() {
		return {
			username: session.username || '',
			savingName: false,
			savingPhoto: false,
			nameError: null,
			photoError: null,
			ok: null,
			accept: 'image/png,image/jpeg,image/gif,image/webp',
		}
	},
	// Values derived from reactive values
	computed: {
		session() {
			return session
		},
		valid() {
			return /^[a-zA-Z0-9_.-]{1,30}$/.test(this.username)
		},
		unchanged() {
			return this.username === session.username
		},
	},
	// Functions used by components
	methods: {
		async saveName() {
			if (!this.valid || this.savingName) return
			this.savingName = true
			this.nameError = null
			this.ok = null
			try {
				const user = await setMyUserName(this.username)
				setUser(user)
				remember(user)
				this.ok = 'Username updated.'
			} catch (e) {
				this.nameError = errorMessage(e)
			} finally {
				this.savingName = false
			}
		},
		async pickPhoto(event) {
			const file = event.target.files && event.target.files[0]
			// The input is reset either way, so picking the same file twice still fires a change
			event.target.value = ''
			if (!file) return

			this.savingPhoto = true
			this.photoError = null
			this.ok = null
			try {
				const user = await setMyPhoto(file)
				setUser(user)
				remember(user)
				this.ok = 'Photo updated.'
			} catch (e) {
				this.photoError = errorMessage(e)
			} finally {
				this.savingPhoto = false
			}
		},
		signOut() {
			// The blobs were fetched with this token and the directory holds this session's users:
			// neither belongs to whoever signs in next
			revokeAll()
			clearUsers()
			logout()
			this.$router.push('/login')
		},
	},
}
</script>

<template>
  <div class="py-3 px-3">
    <div class="d-flex align-items-center justify-content-between border-bottom pb-2 mb-3">
      <h1 class="h4 mb-0">Your profile</h1>
      <RouterLink to="/" class="btn btn-sm btn-outline-secondary">Back to chats</RouterLink>
    </div>

    <div class="profile-body">
      <div v-if="ok" class="alert alert-success" role="alert">{{ ok }}</div>

      <section class="mb-4">
        <h2 class="h6 text-body-secondary text-uppercase">Photo</h2>
        <div class="d-flex align-items-center gap-3">
          <AuthPhoto
            :src="session.photo"
            :alt="session.username"
            :placeholder="session.username"
            :size="96"
          />
          <div>
            <label class="btn btn-outline-primary mb-1">
              {{ savingPhoto ? 'Uploading…' : 'Change photo' }}
              <input
                type="file"
                class="d-none"
                :accept="accept"
                :disabled="savingPhoto"
                @change="pickPhoto"
              >
            </label>
            <div class="form-text">PNG, JPEG, GIF or WEBP, up to 30 MB.</div>
          </div>
        </div>
        <ErrorMsg v-if="photoError" :msg="photoError" class="mt-2" />
      </section>

      <section class="mb-4">
        <h2 class="h6 text-body-secondary text-uppercase">Username</h2>
        <form class="row g-2 align-items-start" @submit.prevent="saveName">
          <div class="col-sm-7">
            <div class="input-group">
              <span class="input-group-text">@</span>
              <input
                v-model.trim="username"
                type="text"
                class="form-control"
                maxlength="30"
                :disabled="savingName"
              >
            </div>
            <p v-if="username && !valid" class="text-danger small mb-0 mt-1">
              1 to 30 characters: letters, digits and <code>_ . -</code>
            </p>
          </div>
          <div class="col-sm-5">
            <button
              type="submit"
              class="btn btn-primary"
              :disabled="!valid || unchanged || savingName"
            >
              {{ savingName ? 'Saving…' : 'Save' }}
            </button>
          </div>
        </form>
        <ErrorMsg v-if="nameError" :msg="nameError" class="mt-2" />
        <p class="form-text">
          Your conversations follow you: the identity is the account, never the name,
          so renaming logs you out of nothing and everyone sees the new name at once.
        </p>
      </section>

      <section class="border-top pt-3">
        <button type="button" class="btn btn-outline-danger" @click="signOut">Sign out</button>
      </section>
    </div>
  </div>
</template>

<style scoped>
.profile-body {
	max-width: 34rem;
}
</style>
