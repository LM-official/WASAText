import { createRouter, createWebHashHistory } from 'vue-router'
import { isLoggedIn } from '../services/session.js'

import LoginView from '../views/LoginView.vue'
import ConversationsView from '../views/ConversationsView.vue'
import ConversationView from '../views/ConversationView.vue'
import NewChatView from '../views/NewChatView.vue'
import NewGroupView from '../views/NewGroupView.vue'
import GroupInfoView from '../views/GroupInfoView.vue'
import ProfileView from '../views/ProfileView.vue'

// Hash history, as the template ships it: the URLs are '#/...', which the Go server does not have to know about.
// That matters for the embedded build, where everything is served under /dashboard/
// by a file server that has no route of its own.
const router = createRouter({
	history: createWebHashHistory(import.meta.env.BASE_URL),
	routes: [
		{ path: '/login', name: 'login', component: LoginView },
		{ path: '/profile', name: 'profile', component: ProfileView },
		{ path: '/', name: 'conversations', component: ConversationsView },
		{ path: '/chats/:chatId', name: 'chat', component: ConversationView },
		// The static path is declared before the parameter one it could be mistaken for
		{ path: '/chats/new', name: 'new-chat', component: NewChatView },
		{ path: '/groups/new', name: 'new-group', component: NewGroupView },
		{ path: '/groups/:groupId/info', name: 'group-info', component: GroupInfoView },
		// Anything else is not a view of this app
		{ path: '/:pathMatch(.*)*', redirect: '/' },
	],
})

// Every operation but doLogin needs the token, so a view without a session has nothing to show
router.beforeEach((to) => {
	// Prevent unlogged users to open any view different than login
	if (!isLoggedIn() && to.name !== 'login') return { name: 'login' }
	// Prevent logged users to open the login view
	if (isLoggedIn() && to.name === 'login') return { name: 'conversations' }
	return true
})

export default router
