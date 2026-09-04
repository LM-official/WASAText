<script>
import { countChars } from '../services/format.js'
import QuotedMessage from './QuotedMessage.vue'

// What writes a message: a text, a photo, or both.
//
// sendMessage refuses a request carrying neither, and a text of spaces alone counts as nothing,
// so the send button follows the same rule instead of letting the server answer 400 for it.
export default {
	// Reusable components
	components: { QuotedMessage },
	// Values received from a parent
	props: {
		disabled: { type: Boolean, default: false },
		sending: { type: Boolean, default: false },
		// The message being answered, owned by the parent: it is set by the Reply action of a bubble
		// and cleared once the answer is on its way, so a failed send keeps the quote
		replyingTo: { type: Object, default: null },
	},
	// Update parents
	emits: ['send', 'cancel-reply'],
	// Reactive state for each component instance
	data() {
		return {
			text: '',
			file: null,
			filePreview: null,
			// Only what the server stores: the type is read from the bytes, so a renamed file is refused there anyway,
			// but offering the right filter avoids the round trip
			accept: 'image/png,image/jpeg,image/gif,image/webp',
			maxChars: 5000,
		}
	},
	// Values derived from reactive values
	computed: {
		// Counted the way the server counts, so the number here is the number it will check
		charCount() {
			return countChars(this.text)
		},
		tooLong() {
			return this.charCount > this.maxChars
		},
		canSend() {
			if (this.disabled || this.sending || this.tooLong) return false
			// At least one of the two, exactly as the server requires
			// || gives back an operand and not a boolean: without !! an attached photo makes this a File
			return !!this.text.trim() || !!this.file
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
		pickFile(event) {
			const file = event.target.files && event.target.files[0]
			this.revokePreview()
			this.file = file || null
			// The local file needs no authenticated fetch: it never left the browser
			this.filePreview = file ? URL.createObjectURL(file) : null
		},
		clearFile() {
			this.revokePreview()
			this.file = null
			this.$refs.fileInput.value = ''
		},
		submit() {
			if (!this.canSend) return
			this.$emit('send', {
				text: this.text.trim(),
				file: this.file,
				replyTo: this.replyingTo ? this.replyingTo.id : null,
			})
		},
		// Called by the parent once the server has answered, so nothing is cleared on a failure
		// and a refused message can be sent again without being retyped
		reset() {
			this.text = ''
			this.clearFile()
		},
		focus() {
			this.$refs.textarea.focus()
		},
	},
}
</script>

<template>
  <form class="composer border-top bg-body p-2" @submit.prevent="submit">
    <div v-if="replyingTo" class="d-flex align-items-start gap-2 mb-2">
      <div class="flex-grow-1 min-width-0">
        <div class="form-text mt-0 mb-1">Replying to</div>
        <QuotedMessage :message="replyingTo" />
      </div>
      <button
        type="button"
        class="btn-close mt-3"
        aria-label="Cancel the reply"
        @click="$emit('cancel-reply')"
      />
    </div>

    <div v-if="filePreview" class="composer-preview mb-2">
      <img :src="filePreview" alt="Photo to send">
      <button
        type="button"
        class="btn btn-sm btn-light border composer-preview-remove"
        title="Remove the photo"
        @click="clearFile"
      >
        &times;
      </button>
    </div>

    <div class="d-flex align-items-end gap-2">
      <label class="btn btn-outline-secondary mb-0" title="Attach a photo">
        <span aria-hidden="true">📎</span>
        <span class="visually-hidden">Attach a photo</span>
        <input
          ref="fileInput"
          type="file"
          class="d-none"
          :accept="accept"
          :disabled="disabled"
          @change="pickFile"
        >
      </label>

      <textarea
        ref="textarea"
        v-model="text"
        class="form-control composer-input"
        rows="1"
        placeholder="Write a message"
        :disabled="disabled"
        @keydown.enter.exact.prevent="submit"
      />

      <button type="submit" class="btn btn-primary" :disabled="!canSend">
        {{ sending ? 'Sending…' : 'Send' }}
      </button>
    </div>

    <div class="d-flex justify-content-between align-items-center mt-1">
      <span class="form-text m-0">Enter sends, Shift+Enter goes to a new line</span>
      <span v-if="charCount > maxChars * 0.9" :class="['form-text m-0', tooLong ? 'text-danger' : '']">
        {{ charCount }} / {{ maxChars }}
      </span>
    </div>
  </form>
</template>

<style scoped>
.min-width-0 {
	min-width: 0;
}

.composer-input {
	resize: none;
	max-height: 8rem;
	overflow-y: auto;
}

.composer-preview {
	position: relative;
	display: inline-block;
}

.composer-preview img {
	max-height: 8rem;
	border-radius: 0.5rem;
	display: block;
}

.composer-preview-remove {
	position: absolute;
	top: 0.25rem;
	right: 0.25rem;
	line-height: 1;
	padding: 0 0.4rem;
}
</style>
