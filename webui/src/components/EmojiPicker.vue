<script>
import emojiDataUrl from 'emoji-picker-element-data/en/emojibase/data.json?url'
import 'emoji-picker-element'

// The reactions a message can carry.
//
// commentMessage takes exactly one grapheme cluster of at most 16 code points,
// so a fixed palette is safely inside the rule:
// 👍 is one code point, ❤️ is two, and the widest one here is far from the limit.
// A free text field would let a user type two symbols and get a 400 back.
export default {
	// Values received from a parent
	props: {
		// The emoji the caller has already reacted with, if any: it is marked and clicking it removes
		mine: { type: String, default: null },
	},
	// Update parents
	emits: ['pick', 'remove'],
	// Reactive state for each component instance
	data() {
		return {
			emojis: ['👍', '❤️', '😂', '😮', '😢', '🙏', '🔥', '🎉', '🍳'],
			showAll: false,
			emojiDataUrl,
		}
	},
	// Functions used by components
	methods: {
		choose(emoji) {
			// Picking the reaction already set means taking it back
			if (emoji === this.mine) this.$emit('remove')
			else this.$emit('pick', emoji)
		},
		chooseFromPicker(event) {
			this.choose(event.detail.unicode)
		},
		toggleAll() {
			this.showAll = !this.showAll
			if (!this.showAll) return
			this.$nextTick(() => {
				const style = document.createElement('style')
				style.textContent = '.favorites { display: none }'
				this.$refs.all.shadowRoot.appendChild(style)
			})
		},
	},
}
</script>

<template>
  <div class="emoji-picker">
    <div class="emoji-suggestions d-flex gap-1 p-1">
      <button
        v-for="emoji in emojis"
        :key="emoji"
        type="button"
        :class="['emoji-btn', { 'emoji-btn-active': emoji === mine }]"
        :title="emoji === mine ? 'Remove your reaction' : `React with ${emoji}`"
        @click="choose(emoji)"
      >
        {{ emoji }}
      </button>
      <button
        type="button"
        class="emoji-btn emoji-more"
        title="More reactions"
        aria-label="More reactions"
        :aria-expanded="showAll"
        @click="toggleAll"
      >
        +
      </button>
    </div>

    <emoji-picker
      v-if="showAll"
      ref="all"
      :data-source="emojiDataUrl"
      class="emoji-all light"
      @emoji-click="chooseFromPicker"
    />
  </div>
</template>

<style scoped>
.emoji-picker {
	--emoji-picker-border-radius: 1.5rem;
}

.emoji-suggestions {
	width: max-content;
	background-color: var(--bs-body-bg);
	border: 1px solid #dee2e6;
	border-radius: var(--emoji-picker-border-radius);
	box-shadow: 0 0.25rem 0.75rem rgba(0, 0, 0, 0.15);
}

.emoji-btn {
	border: 0;
	background: transparent;
	font-size: 1.15rem;
	line-height: 1;
	padding: 0.25rem;
	border-radius: 50%;
	cursor: pointer;
}

.emoji-btn:hover {
	background-color: #f1f3f5;
	transform: scale(1.15);
}

.emoji-btn-active {
	background-color: #cfe2ff;
}

.emoji-more {
	font-weight: 700;
}

.emoji-all {
	--background: var(--bs-body-bg);
	--border-radius: var(--emoji-picker-border-radius);

	display: block;
	width: min(22rem, calc(100vw - 2rem));
	height: 20rem;
	margin-top: 0.25rem;
}
</style>
