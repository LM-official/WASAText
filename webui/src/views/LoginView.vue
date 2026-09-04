<script>
import { doLogin, getUser, errorMessage } from '../services/api.js'
import { setUser } from '../services/session.js'
import { remember } from '../services/users.js'

// The simplified login of the project: a username and no password.
//
// An unknown username registers a new user and logs it in; a known one just logs in.
// The answer is the user id, which is the bearer token of every following request.
// doLogin gives back nothing but that id, so getUser fills in the username and the photo the interface shows.
export default {
	// Reactive state for each component instance
	data() {
		return {
			username: '',
			loading: false,
			errormsg: null,
		}
	},
	// Values derived from reactive values
	computed: {
		// The same rule the server enforces (schemas.Username):
		// refusing it here turns a 400 into an explanation the user can act on
		valid() {
			return /^[a-zA-Z0-9_.-]{1,30}$/.test(this.username)
		},
	},
	// Run after component is shown
	mounted() {
		this.$refs.input.focus()
	},
	// Functions used by components
	methods: {
		async submit() {
			if (!this.valid || this.loading) return
			this.loading = true
			this.errormsg = null
			try {
				const userId = await doLogin(this.username)
				// The token has to be stored before getUser, which is authenticated with it
				setUser({ id: userId, username: this.username, photo: null })
				try {
					const user = await getUser(userId)
					setUser(user)
					remember(user)
				} catch {
					// The session is valid without it:
					// the photo simply shows its placeholder until the profile page is opened
				}
				this.$router.push('/')
			} catch (e) {
				this.errormsg = errorMessage(e)
			} finally {
				this.loading = false
			}
		},
	},
}
</script>

<template>
  <div class="login-page d-flex align-items-center justify-content-center">
    <div class="card shadow-sm login-card">
      <div class="card-body p-4">
        <h1 class="h3 text-center mb-1">WASAText</h1>
        <p class="text-body-secondary text-center mb-4">Sign in with your username</p>

        <form @submit.prevent="submit">
          <div class="mb-3">
            <label for="username" class="form-label">Username</label>
            <div class="input-group">
              <span class="input-group-text">@</span>
              <input
                id="username"
                ref="input"
                v-model.trim="username"
                type="text"
                class="form-control"
                placeholder="e.g. alice"
                maxlength="30"
                autocomplete="username"
                :disabled="loading"
              >
            </div>
            <div class="form-text">
              1 to 30 characters: letters, digits and <code>_ . -</code>.
              A username that does not exist yet is registered on the spot.
            </div>
            <p v-if="username && !valid" class="text-danger small mb-0 mt-1">
              That username does not fit the rule above.
            </p>
          </div>

          <ErrorMsg v-if="errormsg" :msg="errormsg" />

          <button type="submit" class="btn btn-primary w-100" :disabled="!valid || loading">
            {{ loading ? 'Signing in…' : 'Sign in' }}
          </button>
        </form>
      </div>
    </div>
  </div>
</template>

<style scoped>
.login-page {
	min-height: 100vh;
	background-color: #f8f9fa;
	padding: 1rem;
}

.login-card {
	width: 100%;
	max-width: 24rem;
}
</style>
