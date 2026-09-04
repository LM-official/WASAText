<script>
export default {
	// Values received from a parent
	props: { msg: String }
}
</script>

<template>
  <div class="alert alert-danger" role="alert">
    {{ msg }}
  </div>
</template>

<style>
</style>
