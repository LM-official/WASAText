<script>
import ConversationList from '../components/ConversationList.vue'

// The homepage: the list of conversations, most recently active first, each showing the name, the photo,
// the time of the last message and a preview of it.
//
// App.vue leaves its sidebar out on this route, so the list is not drawn twice:
// here it is the page and the whole width belongs to it.
// From any other route the same component is the sidebar,
// which is why the rows and the polling live in ConversationList and not in either place that shows it.
export default {
	// Reusable components
	components: { ConversationList },
}
</script>

<template>
  <div class="home d-flex flex-column">
    <div class="home-header border-bottom">
      <div class="home-inner d-flex align-items-center gap-2 px-3 py-2">
        <h1 class="h5 mb-0 flex-grow-1">Chats</h1>
        <RouterLink to="/chats/new" class="btn btn-sm btn-primary">New chat</RouterLink>
        <RouterLink to="/groups/new" class="btn btn-sm btn-outline-primary">New group</RouterLink>
      </div>
    </div>

    <ConversationList class="flex-grow-1 min-height-0 home-inner w-100" />
  </div>
</template>

<style scoped>
.home {
	height: 100%;
	min-height: 0;
	background-color: #fff;
}

.home-header {
	background-color: #f8f9fa;
}

/* A conversation row is a line of text next to a small photo: stretched across a wide monitor it
   would put the time an arm's length from the name, so both halves share one column */
.home-inner {
	max-width: 46rem;
	margin-inline: auto;
}

.min-height-0 {
	min-height: 0;
}
</style>
