// ExcelTool 前端逻辑
// 约定：所有输出路径都由后端解析成绝对路径并回显，前端只负责收集参数、展示结果。

const state = {
    files: { delete: [], merge: [], join: [] },
    config: { defaultOutputDir: '', version: '' },
    currentPanel: 'delete'
};

// ---------------------------------------------------------------- 初始化

document.addEventListener('DOMContentLoaded', async () => {
    initMenu();
    initDragDrop();
    initShortcuts();
    await loadConfig();
    syncOutputInputs('delete');
    syncOutputInputs('merge');
    syncOutputInputs('join');
    updateStatusBar('就绪');
    addLog('delete', '程序已就绪，输出目录：' + (state.config.defaultOutputDir || '（未获取）'));
});

function initMenu() {
    document.querySelectorAll('.menu-item').forEach(item => {
        item.addEventListener('click', () => switchPanel(item.dataset.panel));
    });

    // 主文件变化时刷新 Sheet 列表
    const mainSel = document.getElementById('merge-main-select');
    if (mainSel) {
        mainSel.addEventListener('change', () => {
            const entry = mainFile();
            if (entry && !entry.sheets.length) refreshMeta(entry, 'merge');
            else updateSheetSelect('merge');
        });
    }
}

function switchPanel(panel) {
    document.querySelectorAll('.menu-item').forEach(i => i.classList.remove('active'));
    const item = document.querySelector('.menu-item[data-panel="' + panel + '"]');
    if (item) item.classList.add('active');

    document.querySelectorAll('.panel').forEach(p => p.classList.remove('active'));
    const target = document.getElementById(panel);
    if (target) target.classList.add('active');

    state.currentPanel = panel;
    const names = { delete: '删除指定行列', merge: '同构文件合并', join: '连接合并' };
    updateStatusBar(names[panel] || panel);
}

function initShortcuts() {
    document.addEventListener('keydown', e => {
        if (!e.ctrlKey) return;
        const map = { '1': 'delete', '2': 'merge', '3': 'join' };
        if (map[e.key]) {
            e.preventDefault();
            switchPanel(map[e.key]);
        }
    });
}

function initDragDrop() {
    ['delete', 'merge', 'join'].forEach(panel => {
        const zone = document.getElementById(panel + '-upload');
        if (!zone) return;
        zone.addEventListener('dragover', e => {
            e.preventDefault();
            zone.classList.add('drag-over');
        });
        zone.addEventListener('dragleave', () => zone.classList.remove('drag-over'));
        zone.addEventListener('drop', e => {
            e.preventDefault();
            zone.classList.remove('drag-over');
            const files = Array.from(e.dataTransfer.files).filter(f => /\.(xlsx|xlsm)$/i.test(f.name));
            if (!files.length) {
                addLog(panel, '只支持 .xlsx / .xlsm 文件');
                return;
            }
            addFilesToList(panel, files);
        });
    });
}

async function loadConfig() {
    try {
        const res = await fetch('/api/config');
        const data = await res.json();
        if (data.status === 'ok') {
            state.config = data;
            const badge = document.getElementById('app-version');
            if (badge && data.version) badge.textContent = data.version;
        }
    } catch (e) {
        console.error('读取配置失败', e);
    }
}

// ---------------------------------------------------------------- 文件列表

function selectFiles(panel) {
    const input = document.createElement('input');
    input.type = 'file';
    input.accept = '.xlsx,.xlsm';
    input.multiple = true;
    input.onchange = e => {
        const files = Array.from(e.target.files);
        if (files.length) addFilesToList(panel, files);
    };
    input.click();
}

function addFiles(panel) {
    selectFiles(panel);
}

function addTable(panel) {
    selectFiles(panel);
}

function addFilesToList(panel, files) {
    const list = state.files[panel];
    let added = 0;
    files.forEach(file => {
        if (list.some(f => f.name === file.name && f.size === file.size)) return;
        list.push({ file, name: file.name, size: file.size, sheets: [], sheet: '', header: [], rows: 0, cols: 0 });
        added++;
    });
    renderFileList(panel);
    addLog(panel, '已添加 ' + added + ' 个文件');

    if (panel === 'join') {
        list.forEach(entry => refreshMeta(entry, panel));
    } else {
        const first = list[0];
        if (first) refreshMeta(first, panel);
    }
}

function renderFileList(panel) {
    const list = state.files[panel];
    const holder = document.getElementById(panel + (panel === 'join' ? '-table-list' : '-file-list'));
    if (holder) {
        holder.innerHTML = '';
        list.forEach((entry, index) => {
            const li = document.createElement('li');
            const label = document.createElement('span');
            label.textContent = entry.name + ' (' + formatFileSize(entry.size) + ')' +
                (entry.rows ? ' · ' + entry.rows + '行×' + entry.cols + '列' : '');
            const del = document.createElement('button');
            del.className = 'btn-small btn-danger';
            del.textContent = '×';
            del.onclick = () => removeFile(panel, index);
            li.appendChild(label);
            li.appendChild(del);
            holder.appendChild(li);
        });
    }

    const count = document.getElementById(panel + '-file-count');
    if (count) count.textContent = list.length ? '已选择 ' + list.length + ' 个文件' : '未选择文件';

    if (panel === 'merge') renderMainSelect();

    updateSheetSelect(panel);
}

function renderMainSelect() {
    const sel = document.getElementById('merge-main-select');
    if (!sel) return;
    const list = state.files.merge;
    const previous = sel.value;
    sel.innerHTML = '';
    list.forEach((entry, index) => {
        const opt = document.createElement('option');
        opt.value = String(index);
        opt.textContent = (index === 0 ? '主文件：' : '') + entry.name;
        sel.appendChild(opt);
    });
    if (previous && Number(previous) < list.length) sel.value = previous;
}

function removeFile(panel, index) {
    state.files[panel].splice(index, 1);
    renderFileList(panel);
    if (panel !== 'join' && index === 0) {
        const first = state.files[panel][0];
        if (first) refreshMeta(first, panel);
    }
    addLog(panel, '已移除文件');
}

function clearFiles(panel) {
    state.files[panel] = [];
    renderFileList(panel);
    clearResults(panel);
    addLog(panel, '已清空文件列表');
}

function clearTables(panel) {
    state.files[panel] = [];
    renderFileList(panel);
    clearResults(panel);
    const body = document.querySelector('#join-rules-table tbody');
    if (body) body.innerHTML = '';
    addLog(panel, '已清空所有表格和规则');
}

// ---------------------------------------------------------------- Sheet 信息

async function refreshMeta(entry, panel) {
    try {
        const form = new FormData();
        form.append('file', entry.file);
        const res = await fetch('/api/sheet-info', { method: 'POST', body: form });
        const data = await res.json();
        if (data.status !== 'ok') throw new Error(data.message || '读取失败');

        entry.sheets = data.sheets || [];
        entry.sheet = data.sheet || (entry.sheets[0] || '');
        entry.header = data.header || [];
        entry.rows = data.rows || 0;
        entry.cols = data.cols || 0;
    } catch (e) {
        console.error('读取 Sheet 信息失败', e);
        entry.sheets = [];
    }
    renderFileList(panel || state.currentPanel);
    refreshAllDatalists();
}

function filesOf(panel) {
    return state.files[panel] || [];
}

function firstFile(panel) {
    return filesOf(panel)[0];
}

function mainFile() {
    const sel = document.getElementById('merge-main-select');
    const list = state.files.merge;
    if (!list.length) return null;
    const idx = sel && sel.value ? Number(sel.value) : 0;
    return list[idx] || list[0];
}

function updateSheetSelect(panel) {
    const sel = document.getElementById(panel + '-sheet-select');
    if (!sel) return;

    const entry = panel === 'merge' ? mainFile() : firstFile(panel);
    const sheets = (entry && entry.sheets) || [];

    if (!sheets.length) {
        sel.innerHTML = '<option>请先选择文件</option>';
        sel.disabled = true;
        setSheetInfo(panel, '请先选择文件');
        return;
    }

    const previous = sel.value;
    sel.innerHTML = '';
    sheets.forEach(name => {
        const opt = document.createElement('option');
        opt.value = name;
        opt.textContent = name;
        sel.appendChild(opt);
    });
    sel.disabled = false;

    if (sheets.includes(previous)) sel.value = previous;
    else if (entry.sheet && sheets.includes(entry.sheet)) sel.value = entry.sheet;

    const dim = entry.rows ? entry.rows + ' 行 × ' + entry.cols + ' 列' : '';
    setSheetInfo(panel, [entry.name, sel.value, dim].filter(Boolean).join(' · '));
}

function setSheetInfo(panel, text) {
    const el = document.getElementById(panel + '-sheet-info');
    if (el) el.textContent = text;
}

function selectedSheet(panel) {
    const sel = document.getElementById(panel + '-sheet-select');
    if (sel && !sel.disabled && sel.value && !sel.value.startsWith('请')) return sel.value;
    return '';
}

// ---------------------------------------------------------------- 输出目录

function outputInput(panel) {
    return document.getElementById(panel + '-output-path');
}

function outputValue(panel) {
    const el = outputInput(panel);
    return el ? el.value.trim() : '';
}

function syncOutputInputs(panel) {
    const el = outputInput(panel);
    if (el && !el.value.trim()) el.placeholder = '留空使用：' + (state.config.defaultOutputDir || '程序目录/output');
}

function useDefaultOutputDir(panel) {
    const el = outputInput(panel);
    if (!el) return;
    el.value = state.config.defaultOutputDir || '';
    addLog(panel, '输出目录已设为默认：' + el.value);
}

async function revealOutputDir(panel) {
    let path = outputValue(panel);
    if (!path) path = state.config.defaultOutputDir || '';
    if (!path) {
        alert('还没有可打开的输出目录');
        return;
    }
    try {
        const res = await fetch('/api/reveal?path=' + encodeURIComponent(path), { method: 'POST' });
        const data = await res.json();
        if (data.status !== 'ok') throw new Error(data.message || '打开失败');
        addLog(panel, '已在文件管理器中打开：' + path);
    } catch (e) {
        addLog(panel, '❌ 打开目录失败：' + e.message);
    }
}

function toggleOutputSettings(mode) {
    const suffix = document.getElementById('delete-file-suffix');
    if (suffix) suffix.classList.toggle('hidden', mode === 'overwrite');
}

// ---------------------------------------------------------------- 删除行列

async function executeDelete() {
    const list = state.files.delete;
    if (!list.length) {
        alert('请先选择文件');
        return;
    }

    const payload = {
        deleteFirstRows: checked('delete-first-rows'),
        firstRowsCount: intValue('first-rows-count'),
        deleteLastRows: checked('delete-last-rows'),
        lastRowsCount: intValue('last-rows-count'),
        deleteCustomRows: checked('delete-custom-rows'),
        customRowsInput: value('custom-rows-input'),
        deleteFirstCols: checked('delete-first-cols'),
        firstColsCount: intValue('first-cols-count'),
        deleteLastCols: checked('delete-last-cols'),
        lastColsCount: intValue('last-cols-count'),
        deleteCustomCols: checked('delete-custom-cols'),
        customColsInput: value('custom-cols-input')
    };

    const any = payload.deleteFirstRows || payload.deleteLastRows || payload.deleteCustomRows ||
        payload.deleteFirstCols || payload.deleteLastCols || payload.deleteCustomCols;
    if (!any) {
        alert('请至少勾选一种删除操作');
        return;
    }

    const form = new FormData();
    list.forEach(entry => form.append('files', entry.file));
    form.append('sheetName', selectedSheet('delete'));
    form.append('outputMode', value('delete-output-mode') || 'newfile');
    form.append('outputDir', outputValue('delete'));
    form.append('suffix', value('delete-suffix'));
    Object.keys(payload).forEach(k => form.append(k, payload[k]));

    addLog('delete', '开始处理 ' + list.length + ' 个文件…');
    setBusy('delete', true);

    try {
        const data = await postForm('/api/delete-rows', form);
        const rows = data.files || [];
        let okCount = 0;
        rows.forEach(item => {
            if (item.error) {
                addLog('delete', '❌ ' + item.input + '：' + item.error);
            } else {
                okCount++;
                addLog('delete', '✅ ' + item.input + '：' + item.before.rows + '行×' + item.before.cols +
                    '列 → ' + item.after.rows + '行×' + item.after.cols + '列');
                addLog('delete', '   输出：' + item.output);
            }
        });
        addLog('delete', '完成：成功 ' + okCount + '/' + rows.length + ' 个文件');
        renderResults('delete', rows);
        updateStatusBar('删除完成');
    } catch (e) {
        addLog('delete', '❌ 处理失败：' + e.message);
        alert('处理失败：' + e.message);
    } finally {
        setBusy('delete', false);
    }
}

// ---------------------------------------------------------------- 同构合并

async function executeMerge() {
    const list = state.files.merge;
    if (list.length < 2) {
        alert('请至少选择两个文件进行合并');
        return;
    }

    const form = new FormData();
    list.forEach(entry => form.append('files', entry.file));
    const sel = document.getElementById('merge-main-select');
    form.append('mainIndex', sel ? sel.value : '0');
    form.append('sheetName', selectedSheet('merge'));
    form.append('outputDir', outputValue('merge'));
    form.append('outputName', value('merge-output-name'));

    addLog('merge', '开始合并 ' + list.length + ' 个文件…');
    setBusy('merge', true);

    try {
        const data = await postForm('/api/merge', form);
        const report = data.report || {};
        addLog('merge', '✅ 合并完成：' + (report.rows || 0) + ' 行 × ' + (report.cols || 0) + ' 列，来自 ' + (report.files || 0) + ' 个文件');
        addLog('merge', '   输出：' + data.output);
        (report.skipped || []).forEach(s => addLog('merge', '⚠️ 跳过：' + s));
        (report.warnings || []).forEach(s => addLog('merge', '⚠️ ' + s));
        renderResults('merge', [{ output: data.output, note: (report.rows || 0) + ' 行 × ' + (report.cols || 0) + ' 列' }]);
        updateStatusBar('合并完成');
    } catch (e) {
        addLog('merge', '❌ 合并失败：' + e.message);
        alert('合并失败：' + e.message);
    } finally {
        setBusy('merge', false);
    }
}

// ---------------------------------------------------------------- 连接合并

function addJoinRule() {
    const body = document.querySelector('#join-rules-table tbody');
    if (!body) return;

    const list = state.files.join;
    if (!list.length) {
        alert('请先添加表格');
        return;
    }

    const tr = document.createElement('tr');
    const listId = 'right-cols-' + Date.now() + '-' + Math.floor(Math.random() * 1000);

    // 左键
    const tdLeft = document.createElement('td');
    const leftInput = document.createElement('input');
    leftInput.type = 'text';
    leftInput.className = 'rule-left-key';
    leftInput.placeholder = '列名';
    leftInput.setAttribute('list', 'join-left-cols');
    tdLeft.appendChild(leftInput);

    // 连接类型
    const tdType = document.createElement('td');
    const typeSel = document.createElement('select');
    typeSel.className = 'rule-type';
    [['inner', 'INNER JOIN'], ['left', 'LEFT JOIN'], ['right', 'RIGHT JOIN'], ['notin', 'NOT IN (A-B)']]
        .forEach(([v, t]) => {
            const o = document.createElement('option');
            o.value = v;
            o.textContent = t;
            typeSel.appendChild(o);
        });
    tdType.appendChild(typeSel);

    // 右表
    const tdTable = document.createElement('td');
    const tableSel = document.createElement('select');
    tableSel.className = 'rule-right-table';
    list.forEach((entry, index) => {
        const o = document.createElement('option');
        o.value = String(index);
        o.textContent = entry.name;
        tableSel.appendChild(o);
    });
    tdTable.appendChild(tableSel);

    // 右表 sheet
    const tdSheet = document.createElement('td');
    const sheetSel = document.createElement('select');
    sheetSel.className = 'rule-right-sheet';
    tdSheet.appendChild(sheetSel);

    // 右键
    const tdKey = document.createElement('td');
    const rightInput = document.createElement('input');
    rightInput.type = 'text';
    rightInput.className = 'rule-right-key';
    rightInput.placeholder = '列名';
    rightInput.setAttribute('list', listId);
    tdKey.appendChild(rightInput);

    const dl = document.createElement('datalist');
    dl.id = listId;
    tdKey.appendChild(dl);

    // 删除
    const tdDel = document.createElement('td');
    const del = document.createElement('button');
    del.className = 'btn-small btn-danger';
    del.textContent = '×';
    del.onclick = () => {
        tr.remove();
        addLog('join', '已删除连接规则');
    };
    tdDel.appendChild(del);

    const fillRight = () => {
        const entry = list[Number(tableSel.value)];
        sheetSel.innerHTML = '';
        (entry && entry.sheets || []).forEach(name => {
            const o = document.createElement('option');
            o.value = name;
            o.textContent = name;
            sheetSel.appendChild(o);
        });
        dl.innerHTML = '';
        (entry && entry.header || []).forEach(name => {
            if (!name) return;
            const o = document.createElement('option');
            o.value = name;
            dl.appendChild(o);
        });
    };
    tableSel.onchange = fillRight;
    fillRight();

    tr.appendChild(tdLeft);
    tr.appendChild(tdType);
    tr.appendChild(tdTable);
    tr.appendChild(tdSheet);
    tr.appendChild(tdKey);
    tr.appendChild(tdDel);
    body.appendChild(tr);

    refreshAllDatalists();
    addLog('join', '已添加连接规则');
}

function collectRules() {
    const rows = Array.from(document.querySelectorAll('#join-rules-table tbody tr'));
    const rules = [];
    for (let i = 0; i < rows.length; i++) {
        const tr = rows[i];
        const leftKey = tr.querySelector('.rule-left-key').value.trim();
        const type = tr.querySelector('.rule-type').value;
        const rightIndex = Number(tr.querySelector('.rule-right-table').value);
        const rightSheet = tr.querySelector('.rule-right-sheet').value;
        const rightKey = tr.querySelector('.rule-right-key').value.trim();

        if (!leftKey || !rightKey) {
            throw new Error('第 ' + (i + 1) + ' 条规则缺少左键或右键');
        }
        rules.push({ leftKey, type, rightIndex, rightKey, rightSheet });
    }
    return rules;
}

async function executeJoin() {
    const list = state.files.join;
    if (list.length < 2) {
        alert('请至少选择两个表格进行连接');
        return;
    }

    let rules;
    try {
        rules = collectRules();
    } catch (e) {
        alert(e.message);
        return;
    }
    if (!rules.length) {
        alert('请至少添加一条连接规则');
        return;
    }

    const spec = {
        baseIndex: 0,
        sheetName: selectedSheet('join'),
        outputFields: value('join-output-fields'),
        outputDir: outputValue('join'),
        outputName: value('join-output-name'),
        rules
    };

    const form = new FormData();
    list.forEach(entry => form.append('files', entry.file));
    form.append('spec', JSON.stringify(spec));

    addLog('join', '开始连接，共 ' + rules.length + ' 条规则…');
    setBusy('join', true);

    try {
        const data = await postForm('/api/join', form);
        const report = data.report || {};
        (report.steps || []).forEach(s => addLog('join', s));
        (report.warnings || []).forEach(s => addLog('join', '⚠️ ' + s));
        addLog('join', '✅ 连接完成：' + (report.rows || 0) + ' 行 × ' + (report.cols || 0) + ' 列');
        addLog('join', '   输出：' + data.output);
        renderResults('join', [{ output: data.output, note: (report.rows || 0) + ' 行 × ' + (report.cols || 0) + ' 列' }]);
        updateStatusBar('连接完成');
    } catch (e) {
        addLog('join', '❌ 连接失败：' + e.message);
        alert('连接失败：' + e.message);
    } finally {
        setBusy('join', false);
    }
}

function refreshAllDatalists() {
    let dl = document.getElementById('join-left-cols');
    if (!dl) {
        dl = document.createElement('datalist');
        dl.id = 'join-left-cols';
        document.body.appendChild(dl);
    }
    const names = new Set();
    state.files.join.forEach(entry => (entry.header || []).forEach(h => h && names.add(h)));
    dl.innerHTML = '';
    names.forEach(name => {
        const o = document.createElement('option');
        o.value = name;
        dl.appendChild(o);
    });
}

// ---------------------------------------------------------------- 结果与通用

async function postForm(url, form) {
    const res = await fetch(url, { method: 'POST', body: form });
    if (!res.ok) throw new Error('HTTP ' + res.status);
    const data = await res.json();
    if (data.status !== 'ok') throw new Error(data.message || '服务返回失败');
    return data;
}

function renderResults(panel, items) {
    const holder = document.getElementById(panel + '-results');
    if (!holder) return;
    holder.innerHTML = '';

    items.forEach(item => {
        if (!item.output) return;
        const li = document.createElement('li');

        const text = document.createElement('span');
        text.textContent = item.output + (item.note ? '（' + item.note + '）' : '');

        const open = document.createElement('button');
        open.className = 'btn-small';
        open.textContent = '打开目录';
        open.onclick = () => revealOutputDir2(item.output);

        const a = document.createElement('a');
        a.className = 'btn-small';
        a.textContent = '下载';
        a.href = '/api/download?path=' + encodeURIComponent(item.output);

        li.appendChild(text);
        li.appendChild(open);
        li.appendChild(a);
        holder.appendChild(li);
    });
}

async function revealOutputDir2(filePath) {
    try {
        const res = await fetch('/api/reveal?path=' + encodeURIComponent(filePath), { method: 'POST' });
        const data = await res.json();
        if (data.status !== 'ok') throw new Error(data.message);
    } catch (e) {
        alert('打开目录失败：' + e.message);
    }
}

function clearResults(panel) {
    const holder = document.getElementById(panel + '-results');
    if (holder) holder.innerHTML = '';
}

function setBusy(panel, busy) {
    const btn = document.querySelector('#' + panel + ' .btn-execute');
    if (!btn) return;
    btn.disabled = busy;
    if (busy) {
        btn.dataset.label = btn.textContent;
        btn.textContent = '处理中…';
    } else if (btn.dataset.label) {
        btn.textContent = btn.dataset.label;
    }
}

function checked(id) {
    const el = document.getElementById(id);
    return !!(el && el.checked);
}

function value(id) {
    const el = document.getElementById(id);
    return el ? el.value.trim() : '';
}

function intValue(id) {
    const n = parseInt(value(id), 10);
    return Number.isFinite(n) ? n : 0;
}

function addLog(panel, message) {
    const el = document.getElementById(panel + '-log');
    if (!el) return;
    const time = new Date().toLocaleTimeString();
    el.value += '[' + time + '] ' + message + '\n';
    el.scrollTop = el.scrollHeight;
}

function copyLog(panel) {
    const el = document.getElementById(panel + '-log');
    if (!el) return;
    el.select();
    try {
        document.execCommand('copy');
        addLog(panel, '日志已复制');
    } catch (e) {
        addLog(panel, '复制失败，请手动选择');
    }
}

function updateStatusBar(text) {
    const el = document.getElementById('status-bar');
    if (el) el.textContent = text;
}

function formatFileSize(bytes) {
    if (!bytes) return '0 B';
    const units = ['B', 'KB', 'MB', 'GB'];
    let i = 0;
    let n = bytes;
    while (n >= 1024 && i < units.length - 1) {
        n /= 1024;
        i++;
    }
    return n.toFixed(n >= 10 || i === 0 ? 0 : 1) + ' ' + units[i];
}

async function exitProgram() {
    if (!confirm('确定要退出程序吗？')) return;
    try {
        await fetch('/api/exit', { method: 'POST' });
    } catch (e) {
        // 忽略：进程已经退出时连接会被重置
    }
    updateStatusBar('程序已退出，可以关闭此页面');
    addLog(state.currentPanel, '程序已退出');
}
