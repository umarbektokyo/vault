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

  // Auth state
  let authEnabled = $state(false);
  let authenticated = $state(false);
  let showLoginModal = $state(false);
  let loginUser = $state('');
  let loginPass = $state('');
  let loginError = $state('');

  // Upload state
  let uploading = $state(false);

  // Secret code state
  let showSecretModal = $state(false);
  let secretCode = $state('');
  let secretTarget = $state(null); // entry to set secret on

  // Unlock state
  let showUnlockModal = $state(false);
  let unlockCode = $state('');
  let unlockTarget = $state(null); // entry to unlock
  let unlockError = $state('');

  // Mkdir state
  let showMkdirModal = $state(false);
  let mkdirName = $state('');
  let mkdirError = $state('');

  // Rename state
  let showRenameModal = $state(false);
  let renameTarget = $state(null);
  let renameName = $state('');
  let renameError = $state('');

  // Move state
  let showMoveModal = $state(false);
  let moveTarget = $state(null);
  let moveDest = $state('/');
  let moveError = $state('');

  // Delete state
  let showDeleteModal = $state(false);
  let deleteTarget = $state(null);

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

  // --- Auth ---
  async function checkAuth() {
    try {
      const res = await fetch('/api/auth');
      const data = await res.json();
      authEnabled = data.authEnabled;
      authenticated = data.authenticated;
    } catch {}
  }

  async function doLogin() {
    loginError = '';
    try {
      const res = await fetch('/api/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username: loginUser, password: loginPass })
      });
      if (!res.ok) {
        const data = await res.json();
        loginError = data.error || 'Login failed';
        return;
      }
      authenticated = true;
      showLoginModal = false;
      loginUser = '';
      loginPass = '';
      browse(currentPath);
    } catch {
      loginError = 'Connection error';
    }
  }

  async function doLogout() {
    await fetch('/api/logout', { method: 'POST' });
    authenticated = false;
    browse(currentPath);
  }

  // --- Upload ---
  function triggerUpload() {
    const input = document.createElement('input');
    input.type = 'file';
    input.multiple = true;
    input.onchange = async () => {
      if (!input.files.length) return;
      uploading = true;
      for (const file of input.files) {
        const form = new FormData();
        form.append('file', file);
        form.append('path', currentPath);
        await fetch('/api/upload', { method: 'POST', body: form });
      }
      uploading = false;
      browse(currentPath);
    };
    input.click();
  }

  // --- Secrets ---
  function openSetSecret(entry) {
    secretTarget = entry;
    secretCode = '';
    showSecretModal = true;
  }

  async function doSetSecret() {
    if (!secretTarget) return;
    await fetch('/api/secret', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ path: secretTarget.path, code: secretCode })
    });
    showSecretModal = false;
    secretCode = '';
    secretTarget = null;
    browse(currentPath);
  }

  async function doRemoveSecret(entry) {
    await fetch('/api/secret', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ path: entry.path, code: '' })
    });
    browse(currentPath);
  }

  // --- Mkdir ---
  function openMkdir() {
    mkdirName = '';
    mkdirError = '';
    showMkdirModal = true;
  }

  async function doMkdir() {
    if (!mkdirName.trim()) return;
    mkdirError = '';
    const res = await fetch('/api/mkdir', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ path: currentPath, name: mkdirName.trim() })
    });
    if (!res.ok) {
      const data = await res.json();
      mkdirError = data.error || 'Failed to create folder';
      return;
    }
    showMkdirModal = false;
    mkdirName = '';
    browse(currentPath);
  }

  // --- Rename ---
  function openRename(entry) {
    renameTarget = entry;
    renameName = entry.name;
    renameError = '';
    showRenameModal = true;
  }

  async function doRename() {
    if (!renameTarget || !renameName.trim()) return;
    renameError = '';
    const res = await fetch('/api/rename', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ path: renameTarget.path, newName: renameName.trim() })
    });
    if (!res.ok) {
      const data = await res.json();
      renameError = data.error || 'Rename failed';
      return;
    }
    showRenameModal = false;
    renameTarget = null;
    renameName = '';
    selectedIndex = -1;
    browse(currentPath);
  }

  // --- Move ---
  function openMove(entry) {
    moveTarget = entry;
    moveDest = currentPath === '/' ? '/' : currentPath;
    moveError = '';
    showMoveModal = true;
  }

  async function doMove() {
    if (!moveTarget || !moveDest.trim()) return;
    moveError = '';
    const res = await fetch('/api/move', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ path: moveTarget.path, dest: moveDest.trim() })
    });
    if (!res.ok) {
      const data = await res.json();
      moveError = data.error || 'Move failed';
      return;
    }
    showMoveModal = false;
    moveTarget = null;
    moveDest = '/';
    selectedIndex = -1;
    browse(currentPath);
  }

  // --- Delete ---
  function openDelete(entry) {
    deleteTarget = entry;
    showDeleteModal = true;
  }

  async function doDelete() {
    if (!deleteTarget) return;
    const res = await fetch('/api/delete', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ path: deleteTarget.path })
    });
    if (!res.ok) return;
    showDeleteModal = false;
    deleteTarget = null;
    selectedIndex = -1;
    browse(currentPath);
  }

  function openUnlock(entry) {
    unlockTarget = entry;
    unlockCode = '';
    unlockError = '';
    showUnlockModal = true;
  }

  async function doUnlock() {
    if (!unlockTarget) return;
    unlockError = '';
    const res = await fetch('/api/unlock', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ path: unlockTarget.path, code: unlockCode })
    });
    if (!res.ok) {
      const data = await res.json();
      unlockError = data.error || 'Failed to unlock';
      return;
    }
    showUnlockModal = false;
    // Now open the file normally
    const entry = unlockTarget;
    unlockTarget = null;
    unlockCode = '';
    const type = getFileType(entry.ext);
    if (type !== 'file') {
      previewFile = { ...entry, type, url: `/files${entry.path}` };
    }
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
    } else if (entry.hasSecret && !authenticated) {
      openUnlock(entry);
    } else {
      const type = getFileType(entry.ext);
      if (type !== 'file') {
        previewFile = { ...entry, type, url: `/files${entry.path}` };
      }
    }
  }

  function handleEntryDblClick(entry) {
    if (!entry.isDir) {
      if (entry.hasSecret && !authenticated) {
        openUnlock(entry);
      } else {
        window.open(`/files${entry.path}`, '_blank');
      }
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
    if (event) event.stopPropagation();
    if (entry.hasSecret && !authenticated) {
      openUnlock(entry);
      return;
    }
    const a = document.createElement('a');
    a.href = `/files${entry.path}`;
    a.download = entry.name;
    a.click();
  }

  function copyUrl(entry, event) {
    if (event) event.stopPropagation();
    navigator.clipboard.writeText(`${window.location.origin}/files${entry.path}`);
  }

  function toggleSection(key) {
    panelSections = { ...panelSections, [key]: !panelSections[key] };
  }

  function getTypeLabel(entry) {
    if (entry.isDir) return 'folder';
    return getFileType(entry.ext);
  }

  checkAuth();
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

      <a class="bw icon-sq github-link" href="https://github.com/umarbektokyo/vault" target="_blank" rel="noopener noreferrer" title="GitHub">
        <svg width="14" height="14" viewBox="0 0 16 16" fill="currentColor"><path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.01 8.01 0 0016 8c0-4.42-3.58-8-8-8z"/></svg>
      </a>

      {#if authEnabled}
        {#if authenticated}
          <button class="bw header-action-btn auth-btn" onclick={doLogout} title="Logout">
            <svg width="11" height="11" viewBox="0 0 11 11"><path d="M4 1.5H2.5a1 1 0 00-1 1v6a1 1 0 001 1H4M7.5 7.5L9.5 5.5 7.5 3.5M9.5 5.5H4" stroke="currentColor" stroke-width="1" fill="none" stroke-linecap="round" stroke-linejoin="round"/></svg>
            <span>Logout</span>
          </button>
        {:else}
          <button class="bw header-action-btn auth-btn" onclick={() => showLoginModal = true} title="Login">
            <svg width="11" height="11" viewBox="0 0 11 11"><circle cx="5.5" cy="3.5" r="2" stroke="currentColor" stroke-width="1" fill="none"/><path d="M1.5 9.5c0-2.2 1.8-4 4-4s4 1.8 4 4" stroke="currentColor" stroke-width="1" fill="none" stroke-linecap="round"/></svg>
            <span>Login</span>
          </button>
        {/if}
      {/if}

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
      {#if authenticated}
        <button class="bw header-action-btn" onclick={openMkdir} title="New Folder">
          <svg width="11" height="11" viewBox="0 0 11 11"><path d="M1.5 2.5h3l1.5 1.5h3.5v5h-8z" stroke="currentColor" stroke-width="0.9" fill="none" stroke-linejoin="round"/><path d="M5.5 5v3M4 6.5h3" stroke="currentColor" stroke-width="0.9" stroke-linecap="round"/></svg>
          <span>New Folder</span>
        </button>
        <button class="bw header-action-btn" onclick={triggerUpload} disabled={uploading} title="Upload files">
          <svg width="11" height="11" viewBox="0 0 11 11"><path d="M5.5 9V3.5M3 5.5l2.5-2.5L8 5.5M1.5 1h8" stroke="currentColor" stroke-width="1.1" fill="none" stroke-linecap="round" stroke-linejoin="round"/></svg>
          <span>{uploading ? 'Uploading...' : 'Upload'}</span>
        </button>
        <div class="header-sep"></div>
      {/if}

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
                {#if entry.hasSecret && !entry.isDir}
                  <div class="lock-icon-wrap">
                    {@html fileIcon(getTypeLabel(entry), 16)}
                    <svg class="lock-badge" width="8" height="8" viewBox="0 0 10 10"><rect x="1" y="4.5" width="8" height="5" rx="1" fill="#e87d0d"/><path d="M3.5 4.5V3a1.5 1.5 0 013 0v1.5" stroke="#e87d0d" stroke-width="1.2" fill="none"/></svg>
                  </div>
                {:else}
                  {@html fileIcon(getTypeLabel(entry), 16)}
                {/if}
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
                {#if entry.hasSecret && !entry.isDir}
                  <div class="grid-lock-overlay">
                    <svg width="20" height="20" viewBox="0 0 10 10"><rect x="1" y="4.5" width="8" height="5" rx="1" fill="#e87d0d"/><path d="M3.5 4.5V3a1.5 1.5 0 013 0v1.5" stroke="#e87d0d" stroke-width="1.2" fill="none"/></svg>
                  </div>
                {/if}
                {#if getTypeLabel(entry) === 'image' && !entry.hasSecret}
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
                {#if getTypeLabel(sel) === 'image' && !sel.hasSecret}
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

                  {#if !sel.isDir && sel.hasSecret}
                    <span class="prop-label">Secret</span>
                    <span class="prop-value" style="color: #e87d0d;">Protected</span>
                  {/if}
                </div>

                <!-- Secret code management (auth only) -->
                {#if authenticated && !sel.isDir}
                  <div class="secret-actions">
                    {#if sel.hasSecret}
                      <button class="prop-link secret-btn" onclick={() => doRemoveSecret(sel)}>
                        <svg width="9" height="9" viewBox="0 0 10 10"><rect x="1" y="4.5" width="8" height="5" rx="1" stroke="#e87d0d" stroke-width="0.8" fill="none"/><path d="M3.5 4.5V3a1.5 1.5 0 013 0v1.5" stroke="#e87d0d" stroke-width="0.8" fill="none"/><path d="M2 2l6 6" stroke="#c44235" stroke-width="1" stroke-linecap="round"/></svg>
                        Remove secret
                      </button>
                    {:else}
                      <button class="prop-link secret-btn" onclick={() => openSetSecret(sel)}>
                        <svg width="9" height="9" viewBox="0 0 10 10"><rect x="1" y="4.5" width="8" height="5" rx="1" stroke="#e87d0d" stroke-width="0.8" fill="none"/><path d="M3.5 4.5V3a1.5 1.5 0 013 0v1.5" stroke="#e87d0d" stroke-width="0.8" fill="none"/></svg>
                        Set secret code
                      </button>
                    {/if}
                  </div>
                {/if}

                <!-- File management actions (auth only) -->
                {#if authenticated}
                  <div class="secret-actions">
                    <button class="prop-link" onclick={() => openRename(sel)}>
                      <svg width="9" height="9" viewBox="0 0 10 10"><path d="M1 8.5h2.5l5-5a1 1 0 00-1.5-1.5l-5 5V8.5z" stroke="currentColor" stroke-width="0.8" fill="none"/></svg>
                      Rename
                    </button>
                    <button class="prop-link" onclick={() => openMove(sel)}>
                      <svg width="9" height="9" viewBox="0 0 10 10"><path d="M1 5h7M6 3l2 2-2 2" stroke="currentColor" stroke-width="0.9" fill="none" stroke-linecap="round" stroke-linejoin="round"/></svg>
                      Move
                    </button>
                    <button class="prop-link" style="color: #c44235;" onclick={() => openDelete(sel)}>
                      <svg width="9" height="9" viewBox="0 0 10 10"><path d="M2 3h6M3.5 3V2h3v1M3.5 3v5.5h3V3" stroke="#c44235" stroke-width="0.8" fill="none" stroke-linejoin="round"/></svg>
                      Delete
                    </button>
                  </div>
                {/if}
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

<!-- ===== LOGIN MODAL ===== -->
{#if showLoginModal}
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="modal-backdrop" onclick={() => showLoginModal = false} onkeydown={(e) => e.key === 'Escape' && (showLoginModal = false)}>
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div class="modal" onclick={(e) => e.stopPropagation()} onkeydown={() => {}}>
      <div class="modal-header">
        <span>Login to Vault</span>
        <button class="bw icon-sq modal-close" onclick={() => showLoginModal = false}>
          <svg width="10" height="10" viewBox="0 0 10 10"><path d="M2 2l6 6M8 2L2 8" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/></svg>
        </button>
      </div>
      <div class="modal-body">
        <form onsubmit={(e) => { e.preventDefault(); doLogin(); }}>
          <label class="modal-label">Username</label>
          <input class="modal-input" type="text" bind:value={loginUser} autocomplete="username" />
          <label class="modal-label">Password</label>
          <input class="modal-input" type="password" bind:value={loginPass} autocomplete="current-password" />
          {#if loginError}
            <div class="modal-error">{loginError}</div>
          {/if}
          <button class="bw modal-submit" type="submit">Login</button>
        </form>
      </div>
    </div>
  </div>
{/if}

<!-- ===== SET SECRET MODAL ===== -->
{#if showSecretModal}
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="modal-backdrop" onclick={() => showSecretModal = false} onkeydown={(e) => e.key === 'Escape' && (showSecretModal = false)}>
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div class="modal" onclick={(e) => e.stopPropagation()} onkeydown={() => {}}>
      <div class="modal-header">
        <span>Set Secret Code</span>
        <button class="bw icon-sq modal-close" onclick={() => showSecretModal = false}>
          <svg width="10" height="10" viewBox="0 0 10 10"><path d="M2 2l6 6M8 2L2 8" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/></svg>
        </button>
      </div>
      <div class="modal-body">
        <form onsubmit={(e) => { e.preventDefault(); doSetSecret(); }}>
          <div class="modal-file-name">{secretTarget?.name}</div>
          <label class="modal-label">Secret Code</label>
          <input class="modal-input" type="password" bind:value={secretCode} placeholder="Enter a secret code" />
          <button class="bw modal-submit" type="submit" disabled={!secretCode}>Set Code</button>
        </form>
      </div>
    </div>
  </div>
{/if}

<!-- ===== UNLOCK MODAL ===== -->
{#if showUnlockModal}
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="modal-backdrop" onclick={() => showUnlockModal = false} onkeydown={(e) => e.key === 'Escape' && (showUnlockModal = false)}>
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div class="modal" onclick={(e) => e.stopPropagation()} onkeydown={() => {}}>
      <div class="modal-header">
        <svg width="14" height="14" viewBox="0 0 10 10"><rect x="1" y="4.5" width="8" height="5" rx="1" fill="#e87d0d"/><path d="M3.5 4.5V3a1.5 1.5 0 013 0v1.5" stroke="#e87d0d" stroke-width="1.2" fill="none"/></svg>
        <span>Unlock File</span>
        <button class="bw icon-sq modal-close" onclick={() => showUnlockModal = false}>
          <svg width="10" height="10" viewBox="0 0 10 10"><path d="M2 2l6 6M8 2L2 8" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/></svg>
        </button>
      </div>
      <div class="modal-body">
        <form onsubmit={(e) => { e.preventDefault(); doUnlock(); }}>
          <div class="modal-file-name">{unlockTarget?.name}</div>
          <label class="modal-label">Enter the secret code to access this file</label>
          <input class="modal-input" type="password" bind:value={unlockCode} placeholder="Secret code" />
          {#if unlockError}
            <div class="modal-error">{unlockError}</div>
          {/if}
          <button class="bw modal-submit" type="submit" disabled={!unlockCode}>Unlock</button>
        </form>
      </div>
    </div>
  </div>
{/if}

<!-- ===== RENAME MODAL ===== -->
{#if showRenameModal}
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="modal-backdrop" onclick={() => showRenameModal = false} onkeydown={(e) => e.key === 'Escape' && (showRenameModal = false)}>
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div class="modal" onclick={(e) => e.stopPropagation()} onkeydown={() => {}}>
      <div class="modal-header">
        <span>Rename</span>
        <button class="bw icon-sq modal-close" onclick={() => showRenameModal = false}>
          <svg width="10" height="10" viewBox="0 0 10 10"><path d="M2 2l6 6M8 2L2 8" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/></svg>
        </button>
      </div>
      <div class="modal-body">
        <form onsubmit={(e) => { e.preventDefault(); doRename(); }}>
          <div class="modal-file-name">{renameTarget?.name}</div>
          <label class="modal-label">New Name</label>
          <input class="modal-input" type="text" bind:value={renameName} />
          {#if renameError}
            <div class="modal-error">{renameError}</div>
          {/if}
          <button class="bw modal-submit" type="submit" disabled={!renameName.trim() || renameName === renameTarget?.name}>Rename</button>
        </form>
      </div>
    </div>
  </div>
{/if}

<!-- ===== MOVE MODAL ===== -->
{#if showMoveModal}
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="modal-backdrop" onclick={() => showMoveModal = false} onkeydown={(e) => e.key === 'Escape' && (showMoveModal = false)}>
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div class="modal" onclick={(e) => e.stopPropagation()} onkeydown={() => {}}>
      <div class="modal-header">
        <span>Move</span>
        <button class="bw icon-sq modal-close" onclick={() => showMoveModal = false}>
          <svg width="10" height="10" viewBox="0 0 10 10"><path d="M2 2l6 6M8 2L2 8" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/></svg>
        </button>
      </div>
      <div class="modal-body">
        <form onsubmit={(e) => { e.preventDefault(); doMove(); }}>
          <div class="modal-file-name">{moveTarget?.name}</div>
          <label class="modal-label">Destination folder path</label>
          <input class="modal-input" type="text" bind:value={moveDest} placeholder="/" />
          {#if moveError}
            <div class="modal-error">{moveError}</div>
          {/if}
          <button class="bw modal-submit" type="submit" disabled={!moveDest.trim()}>Move</button>
        </form>
      </div>
    </div>
  </div>
{/if}

<!-- ===== DELETE MODAL ===== -->
{#if showDeleteModal}
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="modal-backdrop" onclick={() => showDeleteModal = false} onkeydown={(e) => e.key === 'Escape' && (showDeleteModal = false)}>
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div class="modal" onclick={(e) => e.stopPropagation()} onkeydown={() => {}}>
      <div class="modal-header">
        <span>Delete</span>
        <button class="bw icon-sq modal-close" onclick={() => showDeleteModal = false}>
          <svg width="10" height="10" viewBox="0 0 10 10"><path d="M2 2l6 6M8 2L2 8" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/></svg>
        </button>
      </div>
      <div class="modal-body">
        <p style="font-size: 11px; color: #ccc; margin: 0 0 10px;">Are you sure you want to delete <strong>{deleteTarget?.name}</strong>{deleteTarget?.isDir ? ' and all its contents' : ''}? This cannot be undone.</p>
        <div style="display: flex; gap: 6px; justify-content: flex-end;">
          <button class="bw modal-submit" style="background: #333;" onclick={() => showDeleteModal = false}>Cancel</button>
          <button class="bw modal-submit" style="background: #c44235;" onclick={doDelete}>Delete</button>
        </div>
      </div>
    </div>
  </div>
{/if}

<!-- ===== NEW FOLDER MODAL ===== -->
{#if showMkdirModal}
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="modal-backdrop" onclick={() => showMkdirModal = false} onkeydown={(e) => e.key === 'Escape' && (showMkdirModal = false)}>
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div class="modal" onclick={(e) => e.stopPropagation()} onkeydown={() => {}}>
      <div class="modal-header">
        <span>New Folder</span>
        <button class="bw icon-sq modal-close" onclick={() => showMkdirModal = false}>
          <svg width="10" height="10" viewBox="0 0 10 10"><path d="M2 2l6 6M8 2L2 8" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/></svg>
        </button>
      </div>
      <div class="modal-body">
        <form onsubmit={(e) => { e.preventDefault(); doMkdir(); }}>
          <label class="modal-label">Folder Name</label>
          <input class="modal-input" type="text" bind:value={mkdirName} placeholder="New folder" />
          {#if mkdirError}
            <div class="modal-error">{mkdirError}</div>
          {/if}
          <button class="bw modal-submit" type="submit" disabled={!mkdirName.trim()}>Create</button>
        </form>
      </div>
    </div>
  </div>
{/if}

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

  .header-action-btn {
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 3px 8px;
    font-size: 10.5px;
    height: 22px;
    white-space: nowrap;
  }

  .auth-btn {
    font-size: 10px;
  }

  .github-link {
    display: flex;
    align-items: center;
    justify-content: center;
    text-decoration: none;
    color: #999;
  }
  .github-link:hover {
    color: #e6e6e6;
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

  /* Lock icon */
  .lock-icon-wrap {
    position: relative;
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .lock-badge {
    position: absolute;
    bottom: -2px;
    right: -3px;
  }

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
    position: relative;
  }
  .grid-lock-overlay {
    position: absolute;
    top: 0;
    left: 0;
    width: 100%;
    height: 100%;
    display: flex;
    align-items: center;
    justify-content: center;
    background: rgba(0,0,0,0.4);
    z-index: 1;
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
    display: inline-flex;
    align-items: center;
    gap: 3px;
  }
  .prop-link:hover { background: #505050; color: #9cc5f0; }

  /* Secret actions in sidebar */
  .secret-actions {
    margin-top: 8px;
    padding-top: 6px;
    border-top: 1px solid #2e2e2e;
  }
  .secret-btn {
    color: #e87d0d !important;
  }
  .secret-btn:hover {
    color: #f09030 !important;
  }

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

  /* ===== MODALS ===== */
  .modal-backdrop {
    position: fixed;
    top: 0;
    left: 0;
    width: 100%;
    height: 100%;
    background: rgba(0,0,0,0.6);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 100;
  }
  .modal {
    background: #3a3a3a;
    border-radius: 8px;
    width: 300px;
    box-shadow: 0 8px 32px rgba(0,0,0,0.5), 0 0 0 1px #555;
    overflow: hidden;
  }
  .modal-header {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 8px 10px;
    background: #444;
    font-size: 12px;
    font-weight: 500;
    color: #e6e6e6;
    border-bottom: 1px solid #333;
  }
  .modal-close {
    margin-left: auto;
  }
  .modal-body {
    padding: 12px 14px 14px;
  }
  .modal-label {
    display: block;
    font-size: 10px;
    color: #999;
    margin-bottom: 3px;
    margin-top: 8px;
  }
  .modal-label:first-child,
  .modal-file-name + .modal-label {
    margin-top: 0;
  }
  .modal-input {
    width: 100%;
    padding: 5px 8px;
    background: #1e1e1e;
    border: none;
    border-radius: 5px;
    color: #e6e6e6;
    font-size: 12px;
    font-family: 'Inter', sans-serif;
    outline: none;
    box-shadow: inset 0 1px 3px rgba(0,0,0,0.4);
  }
  .modal-input:focus {
    box-shadow: inset 0 1px 3px rgba(0,0,0,0.4), 0 0 0 1px #4b76c2;
  }
  .modal-error {
    color: #c44235;
    font-size: 10px;
    margin-top: 6px;
  }
  .modal-submit {
    margin-top: 12px;
    width: 100%;
    padding: 6px;
    font-size: 11px;
    font-weight: 500;
  }
  .modal-file-name {
    font-size: 11px;
    color: #d0d0d0;
    padding: 4px 0 6px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    border-bottom: 1px solid #333;
    margin-bottom: 4px;
  }

  /* Hide GitHub and auth buttons on mobile */
  @media (max-width: 640px) {
    .github-link { display: none !important; }
    .auth-btn { display: none !important; }
  }
</style>
