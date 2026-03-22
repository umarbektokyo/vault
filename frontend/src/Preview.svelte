<script>
  let { file, onclose } = $props();

  let textContent = $state(null);
  let textLoading = $state(false);

  $effect(() => {
    if (file && file.type === 'text') {
      textLoading = true;
      textContent = null;
      fetch(file.url)
        .then(r => r.text())
        .then(t => { textContent = t; textLoading = false; })
        .catch(() => { textContent = 'Failed to load file.'; textLoading = false; });
    }
  });

  function handleKeydown(e) {
    if (e.key === 'Escape') onclose();
  }

  function downloadFile() {
    const a = document.createElement('a');
    a.href = file.url;
    a.download = file.name;
    a.click();
  }

  function copyUrl() {
    navigator.clipboard.writeText(`${window.location.origin}${file.url}`);
  }
</script>

<svelte:window onkeydown={handleKeydown} />

<div class="pv">
  <!-- Preview header (editor-header style) -->
  <div class="pv-header">
    <div class="pv-title-row">
      <svg width="13" height="13" viewBox="0 0 16 16" fill="none">
        <rect x="2" y="2" width="12" height="12" rx="2" stroke="#5a9fd4" stroke-width="1.2"/>
        <path d="M5 5.5h6M5 8h6M5 10.5h4" stroke="#5a9fd4" stroke-width="0.9" stroke-linecap="round"/>
      </svg>
      <span class="pv-filename" title={file.name}>{file.name}</span>
    </div>
    <div class="pv-actions">
      <button class="bw icon-sq" onclick={copyUrl} title="Copy direct URL">
        <svg width="11" height="11" viewBox="0 0 11 11"><path d="M7 .5H3.5A1.5 1.5 0 002 2v6M4.5 3H9a1.5 1.5 0 011.5 1.5v4.5A1.5 1.5 0 019 10.5H4.5A1.5 1.5 0 013 9V4.5A1.5 1.5 0 014.5 3z" stroke="currentColor" fill="none" stroke-width=".9"/></svg>
      </button>
      <button class="bw icon-sq" onclick={downloadFile} title="Download file">
        <svg width="11" height="11" viewBox="0 0 11 11"><path d="M5.5 1v6M3 5l2.5 2.5L8 5M1.5 9.5h8" stroke="currentColor" stroke-width="1.1" fill="none" stroke-linecap="round" stroke-linejoin="round"/></svg>
      </button>
      <button class="bw icon-sq close-sq" onclick={onclose} title="Close preview (Esc)">
        <svg width="10" height="10" viewBox="0 0 10 10"><path d="M2 2l6 6M8 2L2 8" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/></svg>
      </button>
    </div>
  </div>

  <!-- Preview content -->
  <div class="pv-body">
    {#if file.type === 'image'}
      <div class="pv-canvas">
        <img src={file.url} alt={file.name} class="pv-img" />
      </div>
    {:else if file.type === 'pdf'}
      <iframe src={file.url} title={file.name} class="pv-pdf"></iframe>
    {:else if file.type === 'video'}
      <!-- svelte-ignore a11y_media_has_caption -->
      <video src={file.url} controls class="pv-video"></video>
    {:else if file.type === 'audio'}
      <div class="pv-audio-wrap">
        <svg width="56" height="56" viewBox="0 0 20 20" fill="none">
          <rect x="3" y="2" width="14" height="16" rx="2" stroke="#27ae60" stroke-width="1.2" fill="#27ae60" fill-opacity="0.1"/>
          <path d="M7 8v4M10 6v8M13 8.5v3" stroke="#27ae60" stroke-width="1.3" stroke-linecap="round"/>
        </svg>
        <audio src={file.url} controls class="pv-audio"></audio>
      </div>
    {:else if file.type === 'text'}
      {#if textLoading}
        <div class="pv-loading">Loading...</div>
      {:else}
        <pre class="pv-code">{textContent}</pre>
      {/if}
    {/if}
  </div>
</div>

<style>
  .pv {
    width: 42%;
    min-width: 280px;
    max-width: 600px;
    background: #303030;
    display: flex;
    flex-direction: column;
    flex-shrink: 0;
  }
  .pv-header {
    height: 30px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0 8px;
    background: #303030;
    border-bottom: 1px solid #1a1a1a;
    flex-shrink: 0;
    gap: 8px;
  }
  .pv-title-row {
    display: flex;
    align-items: center;
    gap: 6px;
    min-width: 0;
  }
  .pv-filename {
    font-size: 11px;
    font-weight: 500;
    color: #d0d0d0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .pv-actions {
    display: flex;
    gap: 3px;
    flex-shrink: 0;
  }
  .icon-sq {
    width: 22px;
    height: 22px;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 0;
  }
  .close-sq:hover {
    background: #8b3030 !important;
    box-shadow: 0 1px 0 0 #2a2a2a, inset 0 1px 0 0 #b04040 !important;
  }

  .pv-body {
    flex: 1;
    overflow: auto;
    display: flex;
    align-items: center;
    justify-content: center;
    background: #232323;
    padding: 0;
  }

  /* Checkerboard canvas for images, like Blender's image editor */
  .pv-canvas {
    width: 100%;
    height: 100%;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 12px;
    background-color: #1a1a1a;
    background-image:
      linear-gradient(45deg, #222 25%, transparent 25%),
      linear-gradient(-45deg, #222 25%, transparent 25%),
      linear-gradient(45deg, transparent 75%, #222 75%),
      linear-gradient(-45deg, transparent 75%, #222 75%);
    background-size: 16px 16px;
    background-position: 0 0, 0 8px, 8px -8px, -8px 0;
  }
  .pv-img {
    max-width: 100%;
    max-height: 100%;
    object-fit: contain;
    image-rendering: auto;
  }
  .pv-pdf {
    width: 100%;
    height: 100%;
    border: none;
  }
  .pv-video {
    max-width: 100%;
    max-height: 100%;
  }
  .pv-audio-wrap {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 16px;
  }
  .pv-audio { width: 260px; }
  .pv-code {
    width: 100%;
    height: 100%;
    overflow: auto;
    background: #1e1e1e;
    margin: 0;
    padding: 14px 16px;
    font-family: 'JetBrains Mono', 'SF Mono', 'Fira Code', 'Cascadia Code', monospace;
    font-size: 11px;
    line-height: 1.6;
    color: #ccc;
    white-space: pre;
    tab-size: 4;
    border-top: 1px solid #2a2a2a;
  }
  .pv-loading {
    color: #666;
    font-size: 11px;
  }
</style>
