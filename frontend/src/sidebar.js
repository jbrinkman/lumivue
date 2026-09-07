/**
 * sidebar.js — Camera/stream source list sidebar.
 */

/**
 * Renders the source list into #sidebar.
 * @param {Array} sources           array of Source objects from config
 * @param {string|null} activeId    currently-active source ID
 * @param {(source: object) => void} onSelect  called when user clicks a source
 * @param {(sourceId: string) => void} onSetDefault  called when user right-clicks
 */
export function renderSidebar(sources, activeId, onSelect, onSetDefault) {
  const sidebar = document.getElementById('sidebar');
  if (!sidebar) return;

  const header = `<div class="sidebar-header">Sources</div>`;

  const items = sources.map((src) => {
    const icon = src.type === 'usb' ? '📷' : '📡';
    const label = src.displayName || src.name;
    const activeClass = src.id === activeId ? ' active' : '';
    const defaultClass = src.isDefault ? ' is-default' : '';
    return `
      <div class="source-item${activeClass}${defaultClass}"
           data-id="${escapeAttr(src.id)}"
           tabindex="0"
           title="${escapeAttr(label)}">
        <span class="source-icon">${icon}</span>
        <span class="source-name">${escapeHtml(label)}</span>
        <span class="source-default-dot" title="Default source"></span>
      </div>`;
  }).join('');

  const footer = `
    <div class="sidebar-footer">
      <button class="text-btn" id="add-source-btn" style="width:100%">+ Add Source</button>
    </div>`;

  sidebar.innerHTML = header +
    `<div class="source-list">${items || '<p style="padding:12px;color:var(--text-muted);font-size:12px">No sources yet</p>'}</div>` +
    footer;

  sidebar.querySelectorAll('.source-item').forEach((el) => {
    el.addEventListener('click', () => {
      const src = sources.find((s) => s.id === el.dataset.id);
      if (src) onSelect(src);
    });
    el.addEventListener('keydown', (e) => {
      if (e.key === 'Enter' || e.key === ' ') {
        const src = sources.find((s) => s.id === el.dataset.id);
        if (src) onSelect(src);
      }
    });
    el.addEventListener('contextmenu', (e) => {
      e.preventDefault();
      onSetDefault(el.dataset.id);
    });
  });
}

function escapeHtml(str) {
  return String(str)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;');
}

function escapeAttr(str) {
  return String(str).replace(/"/g, '&quot;');
}
