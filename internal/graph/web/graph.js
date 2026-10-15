// Dataprobe — Relationship Graph Visualizer
(function () {
  'use strict';

  let cy = null;
  let snapshotData = null;
  let selectedSchema = 'all';
  let activeTableId = null;

  // DOM Elements
  const sourceBadge = document.getElementById('sourceBadge');
  const tableCountEl = document.getElementById('tableCount');
  const relationCountEl = document.getElementById('relationCount');
  const schemaCountEl = document.getElementById('schemaCount');
  const schemaFilterList = document.getElementById('schemaFilterList');
  const tableListEl = document.getElementById('tableList');
  const filteredTableCount = document.getElementById('filteredTableCount');
  const tableSearchInput = document.getElementById('tableSearch');
  const graphLoading = document.getElementById('graphLoading');

  // Details Panel Elements
  const detailsPanel = document.getElementById('detailsPanel');
  const panelSchema = document.getElementById('panelSchema');
  const panelTitle = document.getElementById('panelTitle');
  const panelRows = document.getElementById('panelRows');
  const panelDataSize = document.getElementById('panelDataSize');
  const panelIndexSize = document.getElementById('panelIndexSize');
  const colTabCount = document.getElementById('colTabCount');
  const idxTabCount = document.getElementById('idxTabCount');
  const relTabCount = document.getElementById('relTabCount');
  const findTabCount = document.getElementById('findTabCount');
  const columnsTableBody = document.getElementById('columnsTableBody');
  const indexesContainer = document.getElementById('indexesContainer');
  const relationsContainer = document.getElementById('relationsContainer');
  const findingsContainer = document.getElementById('findingsContainer');
  const btnCloseDetails = document.getElementById('btnCloseDetails');
  const btnFit = document.getElementById('btnFit');
  const btnRelayout = document.getElementById('btnRelayout');

  // Format bytes helper
  function formatBytes(bytes) {
    if (!bytes || bytes <= 0) return '0 B';
    const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB'];
    const i = Math.floor(Math.log(bytes) / Math.log(1024));
    return (bytes / Math.pow(1024, i)).toFixed(1) + ' ' + units[i];
  }

  // Format large numbers
  function formatNumber(num) {
    if (num === null || num === undefined) return '-';
    return Number(num).toLocaleString('en-US');
  }

  // Escape HTML helper for safe DOM insertion
  function escapeHTML(str) {
    if (!str) return '';
    return String(str)
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#039;');
  }

  // 1. Fetch snapshot from local authenticated API
  async function init() {
    const params = new URLSearchParams(window.location.search);
    const token = params.get('token') || '';

    try {
      const res = await fetch(`/api/snapshot?token=${encodeURIComponent(token)}`);
      if (!res.ok) {
        throw new Error(`Failed to load snapshot: HTTP ${res.status}`);
      }

      snapshotData = await res.json();
      renderApp();
    } catch (err) {
      graphLoading.textContent = `Error loading database snapshot: ${err.message}`;
    }
  }

  function renderApp() {
    graphLoading.style.display = 'none';

    // Populate top stats
    const tables = snapshotData.tables || [];
    const relations = snapshotData.foreign_keys || [];
    const schemas = snapshotData.source?.schemas || [];

    tableCountEl.textContent = formatNumber(tables.length);
    relationCountEl.textContent = formatNumber(relations.length);
    schemaCountEl.textContent = formatNumber(schemas.length);

    if (snapshotData.source?.alias) {
      sourceBadge.textContent = `${snapshotData.source.engine.toUpperCase()}: ${snapshotData.source.alias}`;
    }

    renderSchemaFilters(schemas);
    renderTableNavList();
    renderCytoscape();
  }

  function renderSchemaFilters(schemas) {
    schemaFilterList.innerHTML = '';

    const allPill = document.createElement('div');
    allPill.className = `filter-pill ${selectedSchema === 'all' ? 'active' : ''}`;
    allPill.textContent = 'All';
    allPill.onclick = () => {
      selectedSchema = 'all';
      updateFilterSelection();
    };
    schemaFilterList.appendChild(allPill);

    schemas.forEach(schema => {
      const pill = document.createElement('div');
      pill.className = `filter-pill ${selectedSchema === schema ? 'active' : ''}`;
      pill.textContent = schema;
      pill.onclick = () => {
        selectedSchema = schema;
        updateFilterSelection();
      };
      schemaFilterList.appendChild(pill);
    });
  }

  function updateFilterSelection() {
    document.querySelectorAll('.filter-pill').forEach(el => {
      if (el.textContent.toLowerCase() === selectedSchema.toLowerCase()) {
        el.classList.add('active');
      } else if (selectedSchema === 'all' && el.textContent === 'All') {
        el.classList.add('active');
      } else {
        el.classList.remove('active');
      }
    });

    renderTableNavList();
    filterGraphNodes();
  }

  function getFilteredTables() {
    const q = tableSearchInput.value.toLowerCase().trim();
    return (snapshotData.tables || []).filter(t => {
      const matchSchema = selectedSchema === 'all' || t.schema.toLowerCase() === selectedSchema.toLowerCase();
      const matchSearch = !q || t.name.toLowerCase().includes(q) || t.schema.toLowerCase().includes(q);
      return matchSchema && matchSearch;
    });
  }

  function renderTableNavList() {
    tableListEl.innerHTML = '';
    const filtered = getFilteredTables();
    filteredTableCount.textContent = filtered.length;

    filtered.forEach(t => {
      const id = `${t.schema}.${t.name}`;
      const item = document.createElement('div');
      item.className = `table-nav-item ${activeTableId === id ? 'active' : ''}`;

      const nameSpan = document.createElement('span');
      nameSpan.textContent = t.name;

      const badgeSpan = document.createElement('span');
      badgeSpan.className = 'table-badge';
      badgeSpan.textContent = `${formatNumber(t.row_count?.value || 0)} rows`;

      item.appendChild(nameSpan);
      item.appendChild(badgeSpan);

      item.onclick = () => {
        selectTable(id);
        focusNodeInGraph(id);
      };

      tableListEl.appendChild(item);
    });
  }

  function renderCytoscape() {
    const elements = [];

    // Table Nodes
    (snapshotData.tables || []).forEach(t => {
      const id = `${t.schema}.${t.name}`;
      elements.push({
        group: 'nodes',
        data: {
          id: id,
          label: t.name,
          schema: t.schema,
          rows: t.row_count?.value || 0,
          dataSize: t.storage_bytes?.data || 0,
        }
      });
    });

    // Foreign Key Edges
    (snapshotData.foreign_keys || []).forEach(fk => {
      const sourceId = `${fk.from.schema}.${fk.from.table}`;
      const targetId = `${fk.to.schema}.${fk.to.table}`;

      elements.push({
        group: 'edges',
        data: {
          id: `${sourceId}->${targetId}:${fk.name}`,
          source: sourceId,
          target: targetId,
          label: fk.name,
          fromCols: fk.from.columns.join(', '),
          toCols: fk.to.columns.join(', ')
        }
      });
    });

    cy = cytoscape({
      container: document.getElementById('cy'),
      elements: elements,
      style: [
        {
          selector: 'node',
          style: {
            'shape': 'round-rectangle',
            'background-color': '#1f2937',
            'border-width': 1.5,
            'border-color': '#4b5563',
            'label': 'data(label)',
            'color': '#f3f4f6',
            'font-family': 'ui-sans-serif, system-ui, sans-serif',
            'font-size': '11px',
            'font-weight': '600',
            'text-valign': 'center',
            'text-halign': 'center',
            'width': 'label',
            'height': '36px',
            'padding': '12px'
          }
        },
        {
          selector: 'node:selected',
          style: {
            'background-color': '#1e3a8a',
            'border-color': '#3b82f6',
            'border-width': 2.5
          }
        },
        {
          selector: 'edge',
          style: {
            'width': 1.5,
            'line-color': '#6b7280',
            'target-arrow-color': '#9ca3af',
            'target-arrow-shape': 'triangle',
            'curve-style': 'bezier',
            'arrow-scale': 0.8
          }
        },
        {
          selector: 'edge:selected',
          style: {
            'width': 2.5,
            'line-color': '#60a5fa',
            'target-arrow-color': '#60a5fa'
          }
        }
      ],
      layout: {
        name: 'breadthfirst',
        directed: true,
        padding: 50,
        spacingFactor: 1.2
      }
    });

    cy.on('tap', 'node', function (evt) {
      const node = evt.target;
      selectTable(node.id());
    });

    cy.on('tap', function (evt) {
      if (evt.target === cy) {
        // Deselect
      }
    });
  }

  function filterGraphNodes() {
    if (!cy) return;
    const filtered = getFilteredTables();
    const allowedIds = new Set(filtered.map(t => `${t.schema}.${t.name}`));

    cy.batch(() => {
      cy.nodes().forEach(node => {
        if (allowedIds.has(node.id())) {
          node.show();
        } else {
          node.hide();
        }
      });
    });
  }

  function focusNodeInGraph(id) {
    if (!cy) return;
    const node = cy.getElementById(id);
    if (node && node.length > 0) {
      cy.center(node);
      cy.nodes().unselect();
      node.select();
    }
  }

  function selectTable(id) {
    activeTableId = id;
    renderTableNavList();

    const [schema, name] = id.split('.');
    const tbl = (snapshotData.tables || []).find(t => t.schema === schema && t.name === name);
    if (!tbl) return;

    detailsPanel.classList.remove('hidden');
    panelSchema.textContent = tbl.schema;
    panelTitle.textContent = tbl.name;
    panelRows.textContent = `${formatNumber(tbl.row_count?.value || 0)} (${tbl.row_count?.quality || 'est'})`;
    panelDataSize.textContent = formatBytes(tbl.storage_bytes?.data || 0);
    panelIndexSize.textContent = formatBytes(tbl.storage_bytes?.indexes || 0);

    // Columns Tab
    colTabCount.textContent = tbl.columns?.length || 0;
    columnsTableBody.innerHTML = '';
    (tbl.columns || []).forEach(col => {
      const tr = document.createElement('tr');
      tr.innerHTML = `
        <td><strong>${escapeHTML(col.name)}</strong></td>
        <td>${escapeHTML(col.db_type)}</td>
        <td>${col.nullable ? 'YES' : 'NO'}</td>
        <td>${col.is_primary_key ? '<span class="pk-badge">PK</span>' : ''}</td>
      `;
      columnsTableBody.appendChild(tr);
    });

    // Indexes Tab
    idxTabCount.textContent = tbl.indexes?.length || 0;
    indexesContainer.innerHTML = '';
    if (!tbl.indexes || tbl.indexes.length === 0) {
      indexesContainer.innerHTML = '<div class="stack-item" style="color:var(--text-subtle);">No indexes declared.</div>';
    } else {
      tbl.indexes.forEach(idx => {
        const item = document.createElement('div');
        item.className = 'stack-item';
        const cols = (idx.columns || []).map(c => c.name).join(', ');
        item.innerHTML = `
          <div class="stack-item-title">
            <span>${escapeHTML(idx.name)}</span>
            ${idx.unique ? '<span class="badge">UNIQUE</span>' : ''}
          </div>
          <div class="stack-item-sub">Columns: (${escapeHTML(cols)})</div>
        `;
        indexesContainer.appendChild(item);
      });
    }

    // Relations Tab
    const rels = (snapshotData.foreign_keys || []).filter(fk =>
      (fk.from.schema === schema && fk.from.table === name) ||
      (fk.to.schema === schema && fk.to.table === name)
    );
    relTabCount.textContent = rels.length;
    relationsContainer.innerHTML = '';
    if (rels.length === 0) {
      relationsContainer.innerHTML = '<div class="stack-item" style="color:var(--text-subtle);">No declared foreign keys.</div>';
    } else {
      rels.forEach(fk => {
        const isOutgoing = fk.from.schema === schema && fk.from.table === name;
        const item = document.createElement('div');
        item.className = 'stack-item';
        item.innerHTML = `
          <div class="stack-item-title">
            <span>${escapeHTML(fk.name)}</span>
            <span class="badge">${isOutgoing ? 'OUTGOING' : 'INCOMING'}</span>
          </div>
          <div class="stack-item-sub">
            ${escapeHTML(fk.from.table)} (${escapeHTML(fk.from.columns.join(', '))})
            &rarr;
            ${escapeHTML(fk.to.table)} (${escapeHTML(fk.to.columns.join(', '))})
          </div>
        `;
        relationsContainer.appendChild(item);
      });
    }

    // Findings Tab
    const tableFindings = (snapshotData.findings || []).filter(f =>
      f.scope?.schema === schema && f.scope?.table === name
    );
    findTabCount.textContent = tableFindings.length;
    findingsContainer.innerHTML = '';
    if (tableFindings.length === 0) {
      findingsContainer.innerHTML = '<div class="stack-item" style="color:var(--text-subtle);">No audit issues detected on this table.</div>';
    } else {
      tableFindings.forEach(f => {
        const item = document.createElement('div');
        item.className = 'stack-item';
        item.innerHTML = `
          <div class="stack-item-title">
            <span>${escapeHTML(f.title)}</span>
            <span class="badge" style="background:#ef4444;color:#fff;">${escapeHTML(f.severity.toUpperCase())}</span>
          </div>
          <div class="stack-item-sub" style="color:#d1d5db;margin-top:4px;">${escapeHTML(f.summary)}</div>
          <div class="stack-item-sub" style="color:#93c5fd;margin-top:4px;"><strong>Advice:</strong> ${escapeHTML(f.recommendation)}</div>
        `;
        findingsContainer.appendChild(item);
      });
    }
  }

  // Event Listeners
  tableSearchInput.addEventListener('input', () => {
    renderTableNavList();
    filterGraphNodes();
  });

  btnCloseDetails.addEventListener('click', () => {
    detailsPanel.classList.add('hidden');
    activeTableId = null;
    renderTableNavList();
    if (cy) cy.nodes().unselect();
  });

  btnFit.addEventListener('click', () => {
    if (cy) cy.fit(undefined, 40);
  });

  btnRelayout.addEventListener('click', () => {
    if (cy) {
      const layout = cy.layout({ name: 'breadthfirst', directed: true, padding: 50, spacingFactor: 1.2 });
      layout.run();
    }
  });

  // Tab switching inside Details panel
  document.querySelectorAll('.tab-btn').forEach(btn => {
    btn.addEventListener('click', () => {
      document.querySelectorAll('.tab-btn').forEach(b => b.classList.remove('active'));
      document.querySelectorAll('.tab-pane').forEach(p => p.classList.remove('active'));

      btn.classList.add('active');
      const targetId = btn.getAttribute('data-tab');
      document.getElementById(targetId)?.classList.add('active');
    });
  });

  // Start initialization
  init();
})();
