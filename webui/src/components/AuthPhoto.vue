<script>
import { photoSrc } from '../services/photos.js'

// Every picture of the app goes through here.
//
// GET /photos/{id} is authenticated, so an <img src="/photos/..."> would be sent without
// the Authorization header and answered 401.
// photos.js fetches the bytes with the token and hands back an object URL,
// which is what this component puts in the src.
export default {
	// Values received from a parent
	props: {
		// The '/photos/<uuid>' the API answered with
		src: { type: String, default: null },
		alt: { type: String, default: '' },
		// Side of the square, in pixels. Ignored when `fluid` is set
		size: { type: Number, default: 40 },
		// A message photo: fills the width of the bubble instead of being a fixed square
		fluid: { type: Boolean, default: false },
		rounded: { type: Boolean, default: true },
		// Shown while the bytes are on their way and when there are none: the initial of the name
		placeholder: { type: String, default: '' },
	},
	// Reactive state for each component instance
	data() {
		return {
			objectURL: null,
			failed: false,
		}
	},
	// Values derived from reactive values
	computed: {
		boxStyle() {
			if (this.fluid) return {}
			return { width: this.size + 'px', height: this.size + 'px' }
		},
		// A fluid photo has no size of its own until the bytes arrive,
		// so the placeholder keeps the bubble from collapsing and then growing under the reader
		placeholderStyle() {
			if (this.fluid) return { width: '100%', height: '8rem', borderRadius: '0.5rem' }
			return this.boxStyle
		},
		initial() {
			return this.placeholder ? this.placeholder.charAt(0).toUpperCase() : ''
		},
	},
	// Run when a reactive value changes
	watch: {
		// A new src is a different photo: the previous object URL belongs to the previous id
		// and the cache in photos.js keeps it for whoever else is showing it
		src: {
			immediate: true,
			handler() {
				this.load()
			},
		},
	},
	// Functions used by components
	methods: {
		async load() {
			this.objectURL = null
			this.failed = false
			if (!this.src) return

			// The src can change again while this one is in flight:
			// only the answer for the src still asked for is allowed to reach the DOM
			const asked = this.src
			try {
				const url = await photoSrc(asked)
				if (this.src === asked) this.objectURL = url
			} catch {
				if (this.src === asked) this.failed = true
			}
		},
	},
}
</script>

<template>
  <img
    v-if="objectURL"
    :src="objectURL"
    :alt="alt"
    :style="boxStyle"
    :class="['auth-photo', { 'rounded-circle': rounded && !fluid, 'auth-photo-fluid': fluid }]"
  >
  <span
    v-else
    :style="placeholderStyle"
    :class="['auth-photo-placeholder', { 'rounded-circle': rounded && !fluid }]"
    :title="failed ? 'Photo unavailable' : alt"
  >{{ initial }}</span>
</template>

<style scoped>
.auth-photo {
	object-fit: cover;
	background-color: #e9ecef;
}

.auth-photo-fluid {
	width: 100%;
	height: auto;
	max-height: 20rem;
	border-radius: 0.5rem;
	display: block;
}

.auth-photo-placeholder {
	display: inline-flex;
	align-items: center;
	justify-content: center;
	background-color: #ced4da;
	color: #495057;
	font-weight: 600;
	flex-shrink: 0;
	overflow: hidden;
}
</style>
