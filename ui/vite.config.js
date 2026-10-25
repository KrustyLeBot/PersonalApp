import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';

export default defineConfig({
  plugins: [svelte()],
  // Relative asset URLs: the same build is served from the Android WebView
  // and from the local server of the Windows app.
  base: './',
  build: {
    outDir: 'dist',
  },
});
