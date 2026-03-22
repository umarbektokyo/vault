<script>
  import FileBrowser from './FileBrowser.svelte';
</script>

<div class="blender-app">
  <div class="blender-main">
    <FileBrowser />
  </div>
</div>

<style>
  @import url('https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600&display=swap');

  :global(*) {
    margin: 0;
    padding: 0;
    box-sizing: border-box;
  }
  :global(body) {
    background: #232323;
    color: #e6e6e6;
    font-family: 'Inter', sans-serif;
    font-size: 11px;
    line-height: 1.35;
    overflow: hidden;
    height: 100vh;
    -webkit-font-smoothing: antialiased;
    -moz-osx-font-smoothing: grayscale;
  }
  :global(#app) {
    height: 100vh;
  }
  :global(::selection) {
    background: #4772b3;
    color: #fff;
  }

  :global(::-webkit-scrollbar) {
    width: 6px;
    height: 6px;
  }
  :global(::-webkit-scrollbar-track) {
    background: transparent;
  }
  :global(::-webkit-scrollbar-thumb) {
    background: #555;
    border-radius: 3px;
  }
  :global(::-webkit-scrollbar-thumb:hover) {
    background: #666;
  }
  :global(::-webkit-scrollbar-corner) {
    background: transparent;
  }

  :global(.bw) {
    font-family: 'Inter', sans-serif;
    font-size: 11px;
    background: #545454;
    border: none;
    border-radius: 6px;
    color: #e6e6e6;
    cursor: pointer;
    outline: none;
    box-shadow: 0 1px 0 0 #2a2a2a, inset 0 1px 0 0 #666;
    transition: background 0.08s;
  }
  :global(.bw:hover) {
    background: #606060;
  }
  :global(.bw:active) {
    background: #4a4a4a;
    box-shadow: 0 1px 0 0 #2a2a2a, inset 0 1px 2px 0 #333;
  }
  :global(.bw:disabled) {
    opacity: 0.3;
    cursor: default;
    pointer-events: none;
  }
  :global(.bw.active) {
    background: #4b76c2;
    box-shadow: 0 1px 0 0 #2a2a2a, inset 0 1px 0 0 #6b96e2;
    color: #fff;
  }

  .blender-app {
    height: 100vh;
    display: flex;
    flex-direction: column;
  }
  .blender-main {
    flex: 1;
    display: flex;
    overflow: hidden;
  }
</style>
