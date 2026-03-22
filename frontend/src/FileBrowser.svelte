<script>
  import Preview from './Preview.svelte';

  // Cookie helpers
  function setCookie(key, value) {
    document.cookie = `vault_${key}=${encodeURIComponent(JSON.stringify(value))};path=/;max-age=31536000;SameSite=Lax`;
  }
  function getCookie(key, fallback) {
    const match = document.cookie.match(new RegExp(`vault_${key}=([^;]+)`));
    if (!match) return fallback;
    try { return JSON.parse(decodeURIComponent(match[1])); } catch { return fallback; }
  }

  let currentPath = $state(getCookie('path', '/'));
  let entries = $state([]);
  let loading = $state(false);
  let error = $state(null);
  let selectedIndex = $state(-1);
  let viewMode = $state(getCookie('viewMode', 'list'));
  let history = $state([]);
  let historyIndex = $state(-1);
  let showSidebar = $state(getCookie('showSidebar', true));
  let filterText = $state('');
  let panelSections = $state(getCookie('panelSections', { system: true, info: true, details: true }));
  let previewFile = $state(null);

  // Persist settings on change
  $effect(() => { setCookie('viewMode', viewMode); });
  $effect(() => { setCookie('showSidebar', showSidebar); });
  $effect(() => { setCookie('panelSections', panelSections); });
  $effect(() => { setCookie('path', currentPath); });

  const imageExts = ['.png', '.jpg', '.jpeg', '.gif', '.webp', '.svg', '.bmp', '.ico', '.avif'];
  const textExts = ['.txt', '.md', '.json', '.xml', '.csv', '.log', '.yml', '.yaml', '.toml', '.ini', '.cfg', '.conf', '.sh', '.bash', '.zsh', '.fish', '.py', '.js', '.ts', '.go', '.rs', '.c', '.cpp', '.h', '.hpp', '.java', '.rb', '.php', '.html', '.css', '.scss', '.less', '.sql', '.r', '.m', '.swift', '.kt', '.lua', '.pl', '.ex', '.exs', '.hs', '.ml', '.vim', '.env', '.gitignore', '.dockerignore', '.editorconfig', '.prettierrc', '.eslintrc', '.babelrc', '.svelte', '.vue', '.jsx', '.tsx', '.mjs', '.cjs', '.lock', '.mod', '.sum', '.makefile'];
  const pdfExts = ['.pdf'];
  const videoExts = ['.mp4', '.webm', '.ogg', '.mov'];
  const audioExts = ['.mp3', '.wav', '.ogg', '.flac', '.aac', '.m4a'];

  let filteredEntries = $derived(
    filterText
      ? entries.filter(e => e.name.toLowerCase().includes(filterText.toLowerCase()))
      : entries
  );

  let dirCount = $derived(entries.filter(e => e.isDir).length);
  let fileCount = $derived(entries.filter(e => !e.isDir).length);
  let totalSize = $derived(entries.reduce((sum, e) => sum + (e.isDir ? 0 : e.size), 0));
  let typeBreakdown = $derived(() => {
    const counts = {};
    for (const e of entries) {
      if (e.isDir) continue;
      const t = getFileType(e.ext);
      counts[t] = (counts[t] || 0) + 1;
    }
    return counts;
  });
  let newestEntry = $derived(entries.length ? entries.reduce((a, b) => new Date(a.modTime) > new Date(b.modTime) ? a : b) : null);
  let selectedEntry = $derived(selectedIndex >= 0 && filteredEntries[selectedIndex] ? filteredEntries[selectedIndex] : null);

  function getFileType(ext) {
    if (imageExts.includes(ext)) return 'image';
    if (pdfExts.includes(ext)) return 'pdf';
    if (textExts.includes(ext)) return 'text';
    if (videoExts.includes(ext)) return 'video';
    if (audioExts.includes(ext)) return 'audio';
    return 'file';
  }

  function formatSize(bytes) {
    if (bytes === 0) return '--';
    const units = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(1024));
    return (bytes / Math.pow(1024, i)).toFixed(i > 0 ? 1 : 0) + ' ' + units[i];
  }

  function formatDate(dateStr) {
    const d = new Date(dateStr);
    return d.toLocaleDateString('en-US', { year: 'numeric', month: 'short', day: 'numeric' }) +
      '  ' + d.toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit', hour12: false });
  }

  async function browse(path, pushHistory = true) {
    loading = true;
    error = null;
    selectedIndex = -1;
    filterText = '';
    try {
      const res = await fetch(`/api/browse?path=${encodeURIComponent(path)}`);
      if (!res.ok) throw new Error('Failed to load directory');
      const data = await res.json();
      currentPath = data.path;
      entries = data.entries;
      if (pushHistory) {
        history = [...history.slice(0, historyIndex + 1), path];
        historyIndex = history.length - 1;
      }
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  function handleEntryClick(entry, index) {
    selectedIndex = index;
    if (entry.isDir) {
      browse(entry.path);
    } else {
      const type = getFileType(entry.ext);
      if (type !== 'file') {
        previewFile = { ...entry, type, url: `/files${entry.path}` };
      }
    }
  }

  function handleEntryDblClick(entry) {
    if (!entry.isDir) {
      window.open(`/files${entry.path}`, '_blank');
    }
  }

  function closePreview() {
    previewFile = null;
  }

  function goUp() {
    if (currentPath === '/') return;
    const parent = currentPath.split('/').slice(0, -1).join('/') || '/';
    browse(parent);
  }

  function goBack() {
    if (historyIndex > 0) { historyIndex--; browse(history[historyIndex], false); }
  }

  function goForward() {
    if (historyIndex < history.length - 1) { historyIndex++; browse(history[historyIndex], false); }
  }

  function goToPath(path) { browse(path); }

  function getBreadcrumbs(path) {
    if (path === '/') return [{ name: 'Root', path: '/' }];
    const parts = path.split('/').filter(Boolean);
    const crumbs = [{ name: 'Root', path: '/' }];
    let acc = '';
    for (const p of parts) { acc += '/' + p; crumbs.push({ name: p, path: acc }); }
    return crumbs;
  }

  function downloadFile(entry, event) {
    event.stopPropagation();
    const a = document.createElement('a');
    a.href = `/files${entry.path}`;
    a.download = entry.name;
    a.click();
  }

  function copyUrl(entry, event) {
    event.stopPropagation();
    navigator.clipboard.writeText(`${window.location.origin}/files${entry.path}`);
  }

  function toggleSection(key) {
    panelSections = { ...panelSections, [key]: !panelSections[key] };
  }

  function getTypeLabel(entry) {
    if (entry.isDir) return 'folder';
    return getFileType(entry.ext);
  }

  browse(currentPath);
</script>

<div class="editor">
  <!-- ===== HEADER BAR ===== -->
  <div class="header-bar">
    <div class="header-left">
      <button class="bw header-type-btn" title="File Browser">
        <svg width="14" height="14" viewBox="0 0 20 20" fill="none">
          <path d="M3 4h5l2 2h7v10H3V4z" stroke="currentColor" stroke-width="1.4" fill="none" stroke-linejoin="round"/>
          <path d="M3 8h14" stroke="currentColor" stroke-width="1"/>
        </svg>
        <span>File Browser</span>
        <svg width="8" height="8" viewBox="0 0 8 8"><path d="M2 3l2 2.5L6 3" stroke="currentColor" stroke-width="1.2" fill="none" stroke-linecap="round" stroke-linejoin="round"/></svg>
      </button>

      <div class="header-sep"></div>

      <button class="bw icon-sq" onclick={goBack} disabled={historyIndex <= 0} title="Back">
        <svg width="12" height="12" viewBox="0 0 12 12"><path d="M7.5 2L3.5 6l4 4" stroke="currentColor" stroke-width="1.5" fill="none" stroke-linecap="round" stroke-linejoin="round"/></svg>
      </button>
      <button class="bw icon-sq" onclick={goForward} disabled={historyIndex >= history.length - 1} title="Forward">
        <svg width="12" height="12" viewBox="0 0 12 12"><path d="M4.5 2l4 4-4 4" stroke="currentColor" stroke-width="1.5" fill="none" stroke-linecap="round" stroke-linejoin="round"/></svg>
      </button>
      <button class="bw icon-sq" onclick={goUp} disabled={currentPath === '/'} title="Parent">
        <svg width="12" height="12" viewBox="0 0 12 12"><path d="M3 7l3-4 3 4" stroke="currentColor" stroke-width="1.5" fill="none" stroke-linecap="round" stroke-linejoin="round"/></svg>
      </button>

      <div class="header-sep"></div>

      <div class="path-widget">
        {#each getBreadcrumbs(currentPath) as crumb, i}
          {#if i > 0}
            <svg class="path-chevron" width="6" height="10" viewBox="0 0 6 10"><path d="M1.5 1l3 4-3 4" stroke="#666" stroke-width="1" fill="none" stroke-linecap="round" stroke-linejoin="round"/></svg>
          {/if}
          <button
            class="path-crumb"
            class:path-current={i === getBreadcrumbs(currentPath).length - 1}
            onclick={() => goToPath(crumb.path)}
          >{crumb.name}</button>
        {/each}
      </div>
    </div>

    <div class="header-right">
      <div class="search-widget">
        <svg class="search-icon" width="11" height="11" viewBox="0 0 11 11"><circle cx="4.5" cy="4.5" r="3" stroke="currentColor" fill="none" stroke-width="1.2"/><path d="M7 7l3 3" stroke="currentColor" stroke-width="1.2" stroke-linecap="round"/></svg>
        <input type="text" bind:value={filterText} placeholder="Filter" class="search-input" />
      </div>

      <div class="header-sep"></div>

      <button class="bw icon-sq" class:active={viewMode === 'list'} onclick={() => viewMode = 'list'} title="List View">
        <svg width="12" height="12" viewBox="0 0 12 12">
          <path d="M1.5 2h9M1.5 6h9M1.5 10h9" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>
        </svg>
      </button>
      <button class="bw icon-sq" class:active={viewMode === 'grid'} onclick={() => viewMode = 'grid'} title="Thumbnail View">
        <svg width="12" height="12" viewBox="0 0 12 12">
          <rect x="1" y="1" width="4" height="4" rx="0.8" stroke="currentColor" fill="none" stroke-width="1.1"/>
          <rect x="7" y="1" width="4" height="4" rx="0.8" stroke="currentColor" fill="none" stroke-width="1.1"/>
          <rect x="1" y="7" width="4" height="4" rx="0.8" stroke="currentColor" fill="none" stroke-width="1.1"/>
          <rect x="7" y="7" width="4" height="4" rx="0.8" stroke="currentColor" fill="none" stroke-width="1.1"/>
        </svg>
      </button>

      <div class="header-sep"></div>

      <button class="bw icon-sq" class:active={showSidebar} onclick={() => showSidebar = !showSidebar} title="Toggle Sidebar (N)">
        <svg width="12" height="12" viewBox="0 0 12 12">
          <rect x="1" y="1.5" width="10" height="9" rx="1.2" stroke="currentColor" fill="none" stroke-width="1.1"/>
          <path d="M8 1.5v9" stroke="currentColor" stroke-width="1"/>
        </svg>
      </button>
    </div>
  </div>

  <div class="editor-body">
    <!-- ===== FILE AREA ===== -->
    <div class="file-area">
      {#if loading}
        <div class="empty-state">Loading...</div>
      {:else if error}
        <div class="empty-state error">{error}</div>
      {:else if filteredEntries.length === 0}
        <div class="empty-state">{filterText ? 'No matches found' : 'Empty directory'}</div>
      {:else if viewMode === 'list'}
        <div class="list-header">
          <div class="lh-icon"></div>
          <div class="lh-name">Name</div>
          <div class="lh-size">Size</div>
          <div class="lh-date">Date Modified</div>
          <div class="lh-act"></div>
        </div>
        <div class="list-body">
          {#each filteredEntries as entry, index}
            <div
              class="list-row"
              class:selected={selectedIndex === index}
              class:even={index % 2 === 0}
              onclick={() => handleEntryClick(entry, index)}
              ondblclick={() => handleEntryDblClick(entry)}
              role="row"
              tabindex="0"
            >
              <div class="lr-icon">
                {@html fileIcon(getTypeLabel(entry), 16)}
              </div>
              <div class="lr-name" class:is-dir={entry.isDir} title={entry.name}>{entry.name}</div>
              <div class="lr-size">{entry.isDir ? '' : formatSize(entry.size)}</div>
              <div class="lr-date">{formatDate(entry.modTime)}</div>
              <div class="lr-act">
                {#if !entry.isDir}
                  <button class="micro-btn" onclick={(e) => copyUrl(entry, e)} title="Copy direct URL">
                    <svg width="10" height="10" viewBox="0 0 10 10"><path d="M6.5.5h-3A1 1 0 002.5 1.5v5M4 2.5h3.5A1 1 0 018.5 3.5v4a1 1 0 01-1 1H4a1 1 0 01-1-1v-4A1 1 0 014 2.5z" stroke="currentColor" fill="none" stroke-width=".9"/></svg>
                  </button>
                  <button class="micro-btn" onclick={(e) => downloadFile(entry, e)} title="Download">
                    <svg width="10" height="10" viewBox="0 0 10 10"><path d="M5 1v5.5m-2-1.5l2 2 2-2M1.5 8.5h7" stroke="currentColor" stroke-width="1" fill="none" stroke-linecap="round" stroke-linejoin="round"/></svg>
                  </button>
                {/if}
              </div>
            </div>
          {/each}
        </div>
      {:else}
        <div class="grid-body">
          {#each filteredEntries as entry, index}
            <button
              class="grid-card"
              class:selected={selectedIndex === index}
              onclick={() => handleEntryClick(entry, index)}
              ondblclick={() => handleEntryDblClick(entry)}
            >
              <div class="grid-thumb-wrap">
                {#if getTypeLabel(entry) === 'image'}
                  <img src={`/files${entry.path}`} alt="" class="grid-thumb-img" loading="lazy" />
                {:else}
                  <div class="grid-thumb-icon">{@html fileIcon(getTypeLabel(entry), 36)}</div>
                {/if}
              </div>
              <div class="grid-label" class:is-dir={entry.isDir}>{entry.name}</div>
            </button>
          {/each}
        </div>
      {/if}
    </div>

    <!-- ===== PREVIEW (between file area and sidebar) ===== -->
    {#if previewFile}
      <div class="region-handle"></div>
      <Preview file={previewFile} onclose={closePreview} />
    {/if}

    <!-- ===== N-PANEL (sidebar) ===== -->
    {#if showSidebar}
      <div class="region-handle"></div>
      <div class="n-panel">
        <!-- System panel -->
        <div class="bp">
          <button class="bp-header" onclick={() => toggleSection('system')}>
            <svg class="bp-disc" class:open={panelSections.system} width="8" height="8" viewBox="0 0 8 8"><path d="M2 1l4 3-4 3z" fill="currentColor"/></svg>
            <svg class="bp-icon" width="13" height="13" viewBox="0 0 16 16"><path d="M2 3h5l2 2h5v8H2V3z" stroke="currentColor" stroke-width="1.2" fill="none" stroke-linejoin="round"/></svg>
            <span>System</span>
          </button>
          {#if panelSections.system}
            <div class="bp-body">
              <button class="nav-item" class:nav-active={currentPath === '/'} onclick={() => goToPath('/')}>
                <svg width="13" height="13" viewBox="0 0 16 16"><path d="M8 2L2 7v7h4v-4h4v4h4V7L8 2z" stroke="currentColor" stroke-width="1.2" fill="none" stroke-linejoin="round"/></svg>
                <span>Root</span>
              </button>
            </div>
          {/if}
        </div>

        <!-- Active Directory panel -->
        <div class="bp">
          <button class="bp-header" onclick={() => toggleSection('info')}>
            <svg class="bp-disc" class:open={panelSections.info} width="8" height="8" viewBox="0 0 8 8"><path d="M2 1l4 3-4 3z" fill="currentColor"/></svg>
            <svg class="bp-icon" width="13" height="13" viewBox="0 0 16 16"><circle cx="8" cy="8" r="6" stroke="currentColor" stroke-width="1.2" fill="none"/><path d="M8 5v1M8 8v3" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/></svg>
            <span>Directory</span>
          </button>
          {#if panelSections.info}
            <div class="bp-body">
              <div class="prop-grid">
                <span class="prop-label">Path</span>
                <span class="prop-value" title={currentPath}>{currentPath}</span>

                <span class="prop-label">Folders</span>
                <span class="prop-value num">{dirCount}</span>

                <span class="prop-label">Files</span>
                <span class="prop-value num">{fileCount}</span>

                <span class="prop-label">Total Items</span>
                <span class="prop-value num">{entries.length}</span>

                <span class="prop-label">Total Size</span>
                <span class="prop-value num">{formatSize(totalSize)}</span>

                {#if newestEntry}
                  <span class="prop-label">Last Modified</span>
                  <span class="prop-value" title={newestEntry.name}>{formatDate(newestEntry.modTime)}</span>
                {/if}
              </div>

              <!-- Type breakdown -->
              {#if fileCount > 0}
                {@const counts = typeBreakdown()}
                <div class="type-breakdown">
                  <div class="breakdown-label">File Types</div>
                  {#each Object.entries(counts).sort((a, b) => b[1] - a[1]) as [type, count]}
                    <div class="breakdown-row">
                      <div class="breakdown-icon">{@html fileIcon(type, 13)}</div>
                      <span class="breakdown-type">{type}</span>
                      <span class="breakdown-count">{count}</span>
                      <div class="breakdown-bar-track">
                        <div class="breakdown-bar-fill" style="width: {(count / fileCount) * 100}%"></div>
                      </div>
                    </div>
                  {/each}
                </div>
              {/if}
            </div>
          {/if}
        </div>

        <!-- Item Details panel (selected or overview) -->
        <div class="bp">
          <button class="bp-header" onclick={() => toggleSection('details')}>
            <svg class="bp-disc" class:open={panelSections.details !== false} width="8" height="8" viewBox="0 0 8 8"><path d="M2 1l4 3-4 3z" fill="currentColor"/></svg>
            <svg class="bp-icon" width="13" height="13" viewBox="0 0 16 16"><rect x="3" y="2" width="10" height="12" rx="1.5" stroke="currentColor" stroke-width="1.2" fill="none"/><path d="M6 5h4M6 8h4M6 11h2" stroke="currentColor" stroke-width="1" stroke-linecap="round"/></svg>
            <span>{selectedEntry ? 'Selected' : 'Item Details'}</span>
          </button>
          {#if panelSections.details !== false}
            <div class="bp-body">
              {#if selectedEntry}
                {@const sel = selectedEntry}
                <!-- Thumbnail preview for images -->
                {#if getTypeLabel(sel) === 'image'}
                  <div class="detail-thumb">
                    <img src={`/files${sel.path}`} alt={sel.name} loading="lazy" />
                  </div>
                {:else}
                  <div class="detail-icon-large">
                    {@html fileIcon(getTypeLabel(sel), 40)}
                  </div>
                {/if}

                <div class="prop-grid">
                  <span class="prop-label">Name</span>
                  <span class="prop-value" title={sel.name}>{sel.name}</span>

                  <span class="prop-label">Type</span>
                  <span class="prop-value type-badge">
                    <span class="badge" class:badge-folder={sel.isDir} class:badge-image={getTypeLabel(sel)==='image'} class:badge-pdf={getTypeLabel(sel)==='pdf'} class:badge-text={getTypeLabel(sel)==='text'} class:badge-video={getTypeLabel(sel)==='video'} class:badge-audio={getTypeLabel(sel)==='audio'}>{getTypeLabel(sel)}</span>
                  </span>

                  {#if !sel.isDir}
                    <span class="prop-label">Size</span>
                    <span class="prop-value num">{formatSize(sel.size)}</span>

                    <span class="prop-label">Extension</span>
                    <span class="prop-value">{sel.ext || 'none'}</span>
                  {/if}

                  <span class="prop-label">Modified</span>
                  <span class="prop-value">{formatDate(sel.modTime)}</span>

                  <span class="prop-label">Path</span>
                  <span class="prop-value" title={sel.path}>{sel.path}</span>

                  {#if !sel.isDir}
                    <span class="prop-label">Direct URL</span>
                    <button class="prop-link" onclick={(e) => copyUrl(sel, e)}>
                      <svg width="9" height="9" viewBox="0 0 10 10"><path d="M6.5.5h-3A1 1 0 002.5 1.5v5M4 2.5h3.5A1 1 0 018.5 3.5v4a1 1 0 01-1 1H4a1 1 0 01-1-1v-4A1 1 0 014 2.5z" stroke="currentColor" fill="none" stroke-width=".9"/></svg>
                      Copy link
                    </button>

                    <span class="prop-label">Download</span>
                    <button class="prop-link" onclick={(e) => downloadFile(sel, e)}>
                      <svg width="9" height="9" viewBox="0 0 10 10"><path d="M5 1v5.5m-2-1.5l2 2 2-2M1.5 8.5h7" stroke="currentColor" stroke-width="1" fill="none" stroke-linecap="round" stroke-linejoin="round"/></svg>
                      Save file
                    </button>
                  {/if}
                </div>
              {:else}
                <!-- Nothing selected: show overview -->
                <div class="detail-empty-hint">
                  <svg width="24" height="24" viewBox="0 0 20 20" fill="none" opacity="0.4">
                    <rect x="3" y="2" width="14" height="16" rx="2" stroke="currentColor" stroke-width="1.2"/>
                    <path d="M6 6h8M6 9h8M6 12h5" stroke="currentColor" stroke-width="1" stroke-linecap="round"/>
                  </svg>
                  <span>Select a file or folder to see details</span>
                </div>
                {#if entries.length > 0}
                  <div class="prop-grid" style="margin-top: 8px;">
                    <span class="prop-label">Largest</span>
                    <span class="prop-value" title={entries.filter(e => !e.isDir).sort((a, b) => b.size - a.size)[0]?.name}>
                      {entries.filter(e => !e.isDir).sort((a, b) => b.size - a.size)[0]?.name || '--'}
                    </span>

                    {#if entries.filter(e => !e.isDir).sort((a, b) => b.size - a.size)[0]}
                      <span class="prop-label"></span>
                      <span class="prop-value num">{formatSize(entries.filter(e => !e.isDir).sort((a, b) => b.size - a.size)[0].size)}</span>
                    {/if}

                    <span class="prop-label">Smallest</span>
                    <span class="prop-value" title={entries.filter(e => !e.isDir && e.size > 0).sort((a, b) => a.size - b.size)[0]?.name}>
                      {entries.filter(e => !e.isDir && e.size > 0).sort((a, b) => a.size - b.size)[0]?.name || '--'}
                    </span>

                    {#if entries.filter(e => !e.isDir && e.size > 0).sort((a, b) => a.size - b.size)[0]}
                      <span class="prop-label"></span>
                      <span class="prop-value num">{formatSize(entries.filter(e => !e.isDir && e.size > 0).sort((a, b) => a.size - b.size)[0].size)}</span>
                    {/if}
                  </div>
                {/if}
              {/if}
            </div>
          {/if}
        </div>
      </div>
    {/if}
  </div>

  <!-- ===== STATUS BAR ===== -->
  <div class="status-bar">
    <span class="sb-text">{filteredEntries.length} item{filteredEntries.length !== 1 ? 's' : ''}{filterText ? ' (filtered)' : ''}</span>
    <span class="sb-path">{currentPath}</span>
  </div>
</div>

<script context="module">
  function fileIcon(type, size) {
    const s = size;
    switch(type) {
      case 'folder':
        return `<svg width="${s}" height="${s}" viewBox="0 0 20 20" fill="none"><path d="M2 5h6l1.5 1.5H18v10H2V5z" stroke="#c89a3c" stroke-width="1.3" fill="#c89a3c" fill-opacity="0.15" stroke-linejoin="round"/><path d="M2 8h16" stroke="#c89a3c" stroke-width="0.8" opacity="0.5"/></svg>`;
      case 'image':
        return `<svg width="${s}" height="${s}" viewBox="0 0 20 20" fill="none"><rect x="2.5" y="3" width="15" height="14" rx="2" stroke="#5a9fd4" stroke-width="1.2" fill="#5a9fd4" fill-opacity="0.1"/><circle cx="7" cy="7.5" r="1.8" stroke="#5a9fd4" stroke-width="1" fill="none"/><path d="M2.5 15l4-4 3 3 2-2 6 3" stroke="#5a9fd4" stroke-width="1" stroke-linejoin="round" fill="none"/></svg>`;
      case 'pdf':
        return `<svg width="${s}" height="${s}" viewBox="0 0 20 20" fill="none"><rect x="3" y="2" width="14" height="16" rx="2" stroke="#c44235" stroke-width="1.2" fill="#c44235" fill-opacity="0.1"/><text x="10" y="13" text-anchor="middle" fill="#c44235" font-size="6" font-weight="600" font-family="Inter,sans-serif" opacity="0.9">PDF</text></svg>`;
      case 'text':
        return `<svg width="${s}" height="${s}" viewBox="0 0 20 20" fill="none"><rect x="3" y="2" width="14" height="16" rx="2" stroke="#999" stroke-width="1.2" fill="#999" fill-opacity="0.06"/><path d="M6 6h8M6 9h8M6 12h5" stroke="#999" stroke-width="1" stroke-linecap="round"/></svg>`;
      case 'video':
        return `<svg width="${s}" height="${s}" viewBox="0 0 20 20" fill="none"><rect x="2" y="4.5" width="16" height="11" rx="2" stroke="#9b59b6" stroke-width="1.2" fill="#9b59b6" fill-opacity="0.1"/><path d="M8 7v6l5-3z" stroke="#9b59b6" stroke-width="1" fill="#9b59b6" fill-opacity="0.2" stroke-linejoin="round"/></svg>`;
      case 'audio':
        return `<svg width="${s}" height="${s}" viewBox="0 0 20 20" fill="none"><rect x="3" y="2" width="14" height="16" rx="2" stroke="#27ae60" stroke-width="1.2" fill="#27ae60" fill-opacity="0.1"/><path d="M7 8v4M10 6v8M13 8.5v3" stroke="#27ae60" stroke-width="1.3" stroke-linecap="round"/></svg>`;
      default:
        return `<svg width="${s}" height="${s}" viewBox="0 0 20 20" fill="none"><rect x="3" y="2" width="14" height="16" rx="2" stroke="#777" stroke-width="1.2" fill="#777" fill-opacity="0.06"/><path d="M6 6h8M6 9h8M6 12h5" stroke="#555" stroke-width="1" stroke-linecap="round"/></svg>`;
    }
  }
</script>

<style>
  .editor {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-width: 0;
    background: #232323;
  }

  /* ===== HEADER BAR ===== */
  .header-bar {
    height: 30px;
    background: #303030;
    border-bottom: 1px solid #1a1a1a;
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0 6px;
    flex-shrink: 0;
    gap: 6px;
  }
  .header-left, .header-right {
    display: flex;
    align-items: center;
    gap: 3px;
  }
  .header-left { flex: 1; min-width: 0; }
  .header-right { flex-shrink: 0; }

  .header-type-btn {
    display: flex;
    align-items: center;
    gap: 5px;
    padding: 3px 8px 3px 6px;
    font-size: 11px;
    font-weight: 500;
    height: 22px;
    white-space: nowrap;
  }

  .icon-sq {
    width: 22px;
    height: 22px;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 0;
  }

  .header-sep {
    width: 1px;
    height: 14px;
    background: #444;
    margin: 0 3px;
    flex-shrink: 0;
  }

  /* ===== PATH WIDGET ===== */
  .path-widget {
    display: flex;
    align-items: center;
    background: #1e1e1e;
    border-radius: 6px;
    padding: 0 4px;
    height: 22px;
    flex: 1;
    min-width: 0;
    overflow-x: auto;
    box-shadow: inset 0 1px 3px rgba(0,0,0,0.4);
  }
  .path-widget::-webkit-scrollbar { height: 0; }
  .path-crumb {
    background: none;
    border: none;
    color: #999;
    cursor: pointer;
    padding: 2px 5px;
    border-radius: 4px;
    font-size: 11px;
    font-family: 'Inter', sans-serif;
    white-space: nowrap;
    flex-shrink: 0;
  }
  .path-crumb:hover { color: #ddd; background: #383838; }
  .path-current { color: #e6e6e6; font-weight: 500; }
  .path-chevron { flex-shrink: 0; margin: 0 1px; }

  /* ===== SEARCH WIDGET ===== */
  .search-widget {
    display: flex;
    align-items: center;
    gap: 4px;
    background: #1e1e1e;
    border-radius: 6px;
    padding: 0 6px;
    height: 22px;
    box-shadow: inset 0 1px 3px rgba(0,0,0,0.4);
  }
  .search-widget:focus-within {
    box-shadow: inset 0 1px 3px rgba(0,0,0,0.4), 0 0 0 1px #4b76c2;
  }
  .search-icon { color: #666; flex-shrink: 0; }
  .search-input {
    background: none;
    border: none;
    outline: none;
    color: #ddd;
    font-size: 11px;
    font-family: 'Inter', sans-serif;
    width: 80px;
  }
  .search-input::placeholder { color: #555; }

  /* ===== EDITOR BODY ===== */
  .editor-body {
    flex: 1;
    display: flex;
    overflow: hidden;
  }

  /* ===== REGION HANDLE (Blender-style panel divider) ===== */
  .region-handle {
    width: 3px;
    background: #1a1a1a;
    cursor: col-resize;
    flex-shrink: 0;
  }
  .region-handle:hover {
    background: #4b76c2;
  }

  /* ===== FILE AREA ===== */
  .file-area {
    flex: 1;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    min-width: 0;
    background: #282828;
  }

  /* List header */
  .list-header {
    display: grid;
    grid-template-columns: 28px 1fr 72px 145px 48px;
    height: 22px;
    align-items: center;
    padding: 0 4px;
    background: #323232;
    border-bottom: 1px solid #1e1e1e;
    position: sticky;
    top: 0;
    z-index: 2;
    font-size: 10px;
    color: #888;
    font-weight: 500;
    letter-spacing: 0.3px;
    text-transform: uppercase;
    flex-shrink: 0;
  }
  .lh-size { text-align: right; padding-right: 8px; }

  /* List body */
  .list-body {
    flex: 1;
    overflow-y: auto;
  }
  .list-row {
    display: grid;
    grid-template-columns: 28px 1fr 72px 145px 48px;
    height: 23px;
    align-items: center;
    padding: 0 4px;
    border: none;
    background: #282828;
    cursor: pointer;
    width: 100%;
    text-align: left;
    font-family: 'Inter', sans-serif;
    font-size: 11px;
    color: #d0d0d0;
    outline: none;
  }
  .list-row.even {
    background: #2c2c2c;
  }
  .list-row:hover {
    background: #353535;
  }
  .list-row.selected {
    background: #264b78;
  }
  .list-row.selected:hover {
    background: #2d5690;
  }
  .list-row:focus-visible {
    box-shadow: inset 0 0 0 1px #4b76c2;
  }
  .lr-icon {
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .lr-name {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    padding-right: 6px;
  }
  .lr-name.is-dir { color: #d4a84b; }
  .lr-size { text-align: right; padding-right: 8px; color: #777; font-size: 10px; font-variant-numeric: tabular-nums; }
  .lr-date { color: #666; font-size: 10px; font-variant-numeric: tabular-nums; }
  .lr-act {
    display: flex;
    gap: 1px;
    justify-content: flex-end;
    opacity: 0;
  }
  .list-row:hover .lr-act { opacity: 1; }
  .micro-btn {
    background: none;
    border: none;
    color: #888;
    cursor: pointer;
    padding: 2px;
    border-radius: 4px;
    display: flex;
    align-items: center;
  }
  .micro-btn:hover { color: #e87d0d; background: #3a3a3a; }

  /* ===== GRID VIEW ===== */
  .grid-body {
    flex: 1;
    overflow-y: auto;
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(88px, 1fr));
    gap: 2px;
    padding: 8px;
    align-content: start;
  }
  .grid-card {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 3px;
    padding: 6px 4px 5px;
    border: 2px solid transparent;
    border-radius: 6px;
    background: none;
    cursor: pointer;
    font-family: 'Inter', sans-serif;
    color: #d0d0d0;
    outline: none;
  }
  .grid-card:hover { background: #333; }
  .grid-card.selected { background: #264b78; border-color: #4b76c2; }
  .grid-thumb-wrap {
    width: 56px;
    height: 56px;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 5px;
    overflow: hidden;
    background: #222;
  }
  .grid-thumb-img {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }
  .grid-thumb-icon {
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .grid-label {
    font-size: 9.5px;
    text-align: center;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    width: 100%;
    padding: 0 2px;
    color: #bbb;
  }
  .grid-label.is-dir { color: #d4a84b; }

  /* ===== N-PANEL (sidebar) ===== */
  .n-panel {
    width: 210px;
    background: #303030;
    overflow-y: auto;
    flex-shrink: 0;
  }

  /* Blender Panel (bp) */
  .bp {
    border-bottom: 1px solid #222;
  }
  .bp-header {
    display: flex;
    align-items: center;
    gap: 5px;
    width: 100%;
    padding: 5px 8px;
    background: #383838;
    border: none;
    color: #ccc;
    cursor: pointer;
    font-family: 'Inter', sans-serif;
    font-size: 11px;
    font-weight: 500;
    text-align: left;
    border-bottom: 1px solid #2a2a2a;
    border-top: 1px solid #444;
  }
  .bp-header:hover { background: #404040; }
  .bp-disc {
    color: #999;
    transition: transform 0.12s;
    flex-shrink: 0;
  }
  .bp-disc.open { transform: rotate(90deg); }
  .bp-icon { color: #999; flex-shrink: 0; }
  .bp-body {
    padding: 6px 10px 8px;
  }

  /* Nav items */
  .nav-item {
    display: flex;
    align-items: center;
    gap: 6px;
    width: 100%;
    padding: 4px 6px;
    background: none;
    border: none;
    border-radius: 5px;
    color: #bbb;
    cursor: pointer;
    font-family: 'Inter', sans-serif;
    font-size: 11px;
    text-align: left;
  }
  .nav-item:hover { background: #3c3c3c; color: #ddd; }
  .nav-item.nav-active {
    background: #4b76c2;
    color: #fff;
    box-shadow: 0 1px 0 0 #2a2a2a, inset 0 1px 0 0 #6b96e2;
    border-radius: 5px;
  }

  /* Property grid */
  .prop-grid {
    display: grid;
    grid-template-columns: auto 1fr;
    gap: 3px 8px;
    align-items: baseline;
  }
  .prop-label {
    color: #888;
    font-size: 10px;
    white-space: nowrap;
  }
  .prop-value {
    color: #ccc;
    font-size: 10.5px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .prop-value.num {
    font-variant-numeric: tabular-nums;
    color: #7ab0e0;
  }
  .prop-link {
    background: #454545;
    border: none;
    border-radius: 4px;
    color: #7ab0e0;
    cursor: pointer;
    font-family: 'Inter', sans-serif;
    font-size: 10px;
    padding: 1px 6px;
    box-shadow: 0 1px 0 0 #2a2a2a, inset 0 1px 0 0 #555;
  }
  .prop-link:hover { background: #505050; color: #9cc5f0; }

  /* ===== STATUS BAR ===== */
  .status-bar {
    height: 20px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0 8px;
    background: #2b2b2b;
    border-top: 1px solid #1a1a1a;
    flex-shrink: 0;
  }
  .sb-text { color: #888; font-size: 10px; }
  .sb-path { color: #555; font-size: 10px; }

  .empty-state {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    color: #555;
    font-size: 12px;
  }
  .empty-state.error { color: #c44235; }

  /* ===== TYPE BREAKDOWN ===== */
  .type-breakdown {
    margin-top: 8px;
    padding-top: 6px;
    border-top: 1px solid #2e2e2e;
  }
  .breakdown-label {
    font-size: 9.5px;
    color: #777;
    text-transform: uppercase;
    letter-spacing: 0.4px;
    margin-bottom: 4px;
  }
  .breakdown-row {
    display: grid;
    grid-template-columns: 16px 1fr auto 40px;
    gap: 4px;
    align-items: center;
    padding: 1.5px 0;
  }
  .breakdown-icon {
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .breakdown-type {
    font-size: 10px;
    color: #aaa;
  }
  .breakdown-count {
    font-size: 10px;
    color: #7ab0e0;
    font-variant-numeric: tabular-nums;
    text-align: right;
  }
  .breakdown-bar-track {
    height: 3px;
    background: #2a2a2a;
    border-radius: 1.5px;
    overflow: hidden;
  }
  .breakdown-bar-fill {
    height: 100%;
    background: #4b76c2;
    border-radius: 1.5px;
    transition: width 0.2s;
  }

  /* ===== DETAIL PANEL EXTRAS ===== */
  .detail-thumb {
    width: 100%;
    aspect-ratio: 16/10;
    border-radius: 5px;
    overflow: hidden;
    background-color: #1a1a1a;
    background-image:
      linear-gradient(45deg, #222 25%, transparent 25%),
      linear-gradient(-45deg, #222 25%, transparent 25%),
      linear-gradient(45deg, transparent 75%, #222 75%),
      linear-gradient(-45deg, transparent 75%, #222 75%);
    background-size: 10px 10px;
    background-position: 0 0, 0 5px, 5px -5px, -5px 0;
    margin-bottom: 8px;
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .detail-thumb img {
    max-width: 100%;
    max-height: 100%;
    object-fit: contain;
  }
  .detail-icon-large {
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 8px 0 10px;
  }
  .detail-empty-hint {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 6px;
    padding: 10px 0 4px;
    color: #555;
    font-size: 10px;
    text-align: center;
  }

  /* Type badge */
  .type-badge { display: flex; }
  .badge {
    font-size: 9px;
    padding: 1px 6px;
    border-radius: 3px;
    background: #3a3a3a;
    color: #aaa;
    text-transform: uppercase;
    letter-spacing: 0.3px;
    font-weight: 500;
  }
  .badge-folder { background: #3d3221; color: #d4a84b; }
  .badge-image { background: #1e3044; color: #5a9fd4; }
  .badge-pdf { background: #3a1e1a; color: #c44235; }
  .badge-text { background: #2e2e2e; color: #999; }
  .badge-video { background: #2e1e3a; color: #9b59b6; }
  .badge-audio { background: #1a2e22; color: #27ae60; }
</style>
