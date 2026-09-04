<script>
import { session, isLoggedIn } from './services/session.js'
import ConversationList from './components/ConversationList.vue'
import AuthPhoto from './components/AuthPhoto.vue'

// The shell: a fixed navbar, the conversations list on the left and the opened view on the right.
//
// The list lives here rather than inside a view because it has to survive a navigation:
// it keeps polling while a group is being created or a profile edited, and switching chat never rebuilds it.
//
// The home route is the exception. There the conversations list is the page itself,
// as the project spec describes the homepage, so the sidebar would only be the same list twice:
// it is left out and the view takes the whole width.
export default {
	// Reusable components
	components: { ConversationList, AuthPhoto },
	// Values derived from reactive values
	computed: {
		session() {
			return session
		},
		// The login page is the whole window: no sidebar to show, and nothing to put in it
		bare() {
			return !isLoggedIn() || this.$route.name === 'login'
		},
		// The home route draws the list itself, so the sidebar beside it would be a duplicate
		showSidebar() {
			return !this.bare && this.$route.name !== 'conversations'
		},
		// Vite rewrites root-relative URLs inside index.html for the --base=/dashboard/ build,
    // but not inside a template: the sprite has to be addressed through the base by hand
		baseUrl() {
			return import.meta.env.BASE_URL
		},
	},
	// Functions used by components
	methods: {
		icon(name) {
			return `${this.baseUrl}feather-sprite-v4.29.0.svg#${name}`
		},
	},
}
</script>

<template>
  <RouterView v-if="bare" />

  <template v-else>
    <header class="navbar sticky-top bg-dark flex-md-nowrap p-0 shadow" data-bs-theme="dark">
      <RouterLink class="navbar-brand col-md-4 col-lg-3 me-0 px-3 fs-6 text-white" to="/">
        WASAText
      </RouterLink>

      <button
        v-if="showSidebar"
        class="navbar-toggler d-md-none"
        type="button"
        data-bs-toggle="collapse"
        data-bs-target="#sidebarMenu"
        aria-controls="sidebarMenu"
        aria-expanded="false"
        aria-label="Toggle the conversations"
      >
        <span class="navbar-toggler-icon" />
      </button>

      <div class="ms-auto d-flex align-items-center gap-2 px-3">
        <RouterLink
          to="/profile"
          class="d-flex align-items-center gap-2 text-white text-decoration-none"
          title="Your profile"
        >
          <AuthPhoto
            :src="session.photo"
            :alt="session.username"
            :placeholder="session.username"
            :size="28"
          />
          <span class="d-none d-sm-inline">{{ session.username }}</span>
        </RouterLink>
      </div>
    </header>

    <div class="container-fluid">
      <div class="row app-row">
        <!-- Not the `sidebar` class of assets/dashboard.css: that one is `position: fixed` with a 48px top padding,
             which is the wrong shape for a list that scrolls inside a column -->
        <nav
          v-if="showSidebar"
          id="sidebarMenu"
          class="col-md-4 col-lg-3 d-md-block bg-light chat-sidebar collapse"
        >
          <div class="sidebar-inner d-flex flex-column">
            <div class="d-flex gap-2 p-2 border-bottom">
              <RouterLink to="/chats/new" class="btn btn-sm btn-primary flex-grow-1">
                <svg class="feather" aria-hidden="true"><use :href="icon('message-square')" /></svg>
                New chat
              </RouterLink>
              <RouterLink to="/groups/new" class="btn btn-sm btn-outline-primary flex-grow-1">
                <svg class="feather" aria-hidden="true"><use :href="icon('users')" /></svg>
                New group
              </RouterLink>
            </div>

            <ConversationList class="flex-grow-1 min-height-0" />
          </div>
        </nav>

        <main :class="['px-0 main-pane', showSidebar ? 'col-md-8 col-lg-9 ms-sm-auto' : 'col-12']">
          <RouterView />
        </main>
      </div>
    </div>
  </template>
</template>

<style scoped>
/* The navbar is fixed and 48px tall (assets/dashboard.css): both panes fill what is left,
  and each scrolls on its own, so the page itself never does */
.app-row {
	height: calc(100vh - 48px);
}

.chat-sidebar {
	padding: 0;
	height: 100%;
}

.sidebar-inner {
	height: 100%;
	min-height: 0;
	border-right: 1px solid #dee2e6;
}

.min-height-0 {
	min-height: 0;
}

.main-pane {
	height: 100%;
	min-height: 0;
	overflow-y: auto;
}

/* Collapsed on a phone, the sidebar is a panel above the view rather than a column beside it */
@media (max-width: 767.98px) {
	.app-row {
		height: auto;
	}

	.sidebar-inner {
		border-right: 0;
		max-height: 70vh;
	}

	.main-pane {
		height: calc(100vh - 48px);
	}
}
</style>
