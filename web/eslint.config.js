import js from '@eslint/js';
import globals from 'globals';
import svelte from 'eslint-plugin-svelte';

export default [
  { ignores: ['dist', 'node_modules'] },
  js.configs.recommended,
  ...svelte.configs['flat/recommended'],
  {
    files: ['**/*.{js,svelte}'],
    languageOptions: { globals: { ...globals.browser } },
  },
  // Svelte runes modules/components: $state, $derived, ... are compiler globals.
  {
    files: ['**/*.svelte', '**/*.svelte.js'],
    languageOptions: { globals: { $state: 'readonly', $derived: 'readonly', $effect: 'readonly', $props: 'readonly', $bindable: 'readonly', $inspect: 'readonly', $host: 'readonly' } },
  },
  // public/ scripts are classic (non-module) scripts served as-is.
  { files: ['public/**/*.js'], languageOptions: { sourceType: 'script', globals: { ...globals.browser } } },
  { files: ['*.config.js', 'scripts/**/*.mjs'], languageOptions: { globals: { ...globals.node } } },
];
