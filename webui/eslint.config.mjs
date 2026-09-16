/*
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * @author ENDERZOMBI102 <enderzombi102.end@gmail.com> 2024
 * @description Quick and dirty `eslint` config to better conform to the Prof's requests and style.
 */
import vue from 'eslint-plugin-vue';

// noinspection JSUnusedGlobalSymbols
export default [
	// Code this project did not write: the Bootstrap bundles, the build output and Yarn's own
	// runtime. Every problem reported outside src comes from these, and none of them is ours to fix
	{
		ignores: [
			'dist/**',
			'public/bootstrap/**',
			'.yarn/**',
			'.pnp.cjs',
			'.pnp.loader.mjs'
		]
	},
	... vue.configs[ "flat/recommended" ],
	{
		rules: {
			'vue/multi-word-component-names': 'off',
			'vue/max-attributes-per-line': 'off',
			'vue/require-default-prop': 'off',
			'vue/singleline-html-element-content-newline': 'off'
		}
	},
];