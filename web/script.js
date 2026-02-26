// 全局变量
let currentPanel = 'delete';
let fileLists = {
    delete: [],
    merge: [],
    join: []
};

// 页面加载完成后初始化
document.addEventListener('DOMContentLoaded', function() {
    initializeMenu();
    initializeEventListeners();
    updateStatusBar('就绪');
});

// 初始化菜单
function initializeMenu() {
    const menuItems = document.querySelectorAll('.menu-item');
    menuItems.forEach(item => {
        item.addEventListener('click', function() {
            const panelId = this.dataset.panel;
            switchPanel(panelId);
        });
    });
}

// 初始化事件监听器
function initializeEventListeners() {
    // 文件拖拽支持
    const fileAreas = document.querySelectorAll('[id$="-upload"]');
    fileAreas.forEach(area => {
        setupDragAndDrop(area);
    });
}

// 切换面板
function switchPanel(panelId) {
    // 更新菜单状态
    document.querySelectorAll('.menu-item').forEach(item => {
        item.classList.remove('active');
    });
    document.querySelector(`[data-panel="${panelId}"]`).classList.add('active');
    
    // 显示对应面板
    document.querySelectorAll('.panel').forEach(panel => {
        panel.classList.remove('active');
    });
    document.getElementById(panelId).classList.add('active');
    
    // 更新状态栏
    const panelNames = {
        'delete': '删除指定行列',
        'merge': '同构文件合并',
        'join': '连接合并'
    };
    updateStatusBar(panelNames[panelId]);
    
    currentPanel = panelId;
}

// 更新状态栏
function updateStatusBar(message) {
    const statusBar = document.getElementById('status-bar');
    statusBar.textContent = message;
}

// 文件选择功能
function selectFiles(panel) {
    const input = document.createElement('input');
    input.type = 'file';
    input.accept = '.xlsx,.xls';
    input.multiple = true;
    input.onchange = function(e) {
        const files = Array.from(e.target.files);
        addFilesToList(panel, files);
        // 自动获取第一个文件的sheet信息
        if (files.length > 0) {
            getSheetInfo(panel, files[0]);
        }
    };
    input.click();
}

function selectFolder(panel) {
    // 正确的文件夹选择功能
    const input = document.createElement('input');
    input.type = 'file';
    input.webkitdirectory = true;
    input.directory = true;
    input.multiple = false;
    input.onchange = function(e) {
        if (e.target.files.length > 0) {
            // 获取文件夹路径
            const folderPath = e.target.files[0].webkitRelativePath.split('/')[0];
            const pathElement = document.getElementById(`${panel}-output-path`);
            if (pathElement) {
                pathElement.value = folderPath;
            }
            addLog(panel, `已选择输出文件夹: ${folderPath}`);
        }
    };
    input.click();
}

function addFiles(panel) {
    selectFiles(panel);
}

function clearFiles(panel) {
    fileLists[panel] = [];
    updateFileList(panel);
    updateFileCount(panel);
    addLog(panel, '已清空文件列表');
}

function clearTables(panel) {
    fileLists[panel] = [];
    updateFileList(panel);
    addLog(panel, '已清空所有表格');
}

// 添加文件到列表
function addFilesToList(panel, files) {
    files.forEach(file => {
        if (!fileLists[panel].some(f => f.name === file.name)) {
            fileLists[panel].push(file);
        }
    });
    updateFileList(panel);
    updateFileCount(panel);
    addLog(panel, `已添加 ${files.length} 个文件`);
}

// 更新文件列表显示
function updateFileList(panel) {
    const listElement = document.getElementById(`${panel}-file-list`);
    if (!listElement) return;
    
    listElement.innerHTML = '';
    fileLists[panel].forEach((file, index) => {
        const li = document.createElement('li');
        li.innerHTML = `
            <span>${file.name} (${formatFileSize(file.size)})</span>
            <button class="btn-small btn-danger" onclick="removeFile('${panel}', ${index})">×</button>
        `;
        listElement.appendChild(li);
    });
}

// 更新文件计数
function updateFileCount(panel) {
    const countElement = document.getElementById(`${panel}-file-count`);
    if (countElement) {
        const count = fileLists[panel].length;
        countElement.textContent = count > 0 ? `已选择 ${count} 个文件` : '未选择文件';
    }
}

// 移除文件
function removeFile(panel, index) {
    fileLists[panel].splice(index, 1);
    updateFileList(panel);
    updateFileCount(panel);
    addLog(panel, '已删除文件');
}

// 拖拽支持
function setupDragAndDrop(element) {
    element.addEventListener('dragover', function(e) {
        e.preventDefault();
        this.style.borderColor = '#4CAF50';
        this.style.backgroundColor = '#f0f8f0';
    });
    
    element.addEventListener('dragleave', function(e) {
        e.preventDefault();
        this.style.borderColor = '#ddd';
        this.style.backgroundColor = 'transparent';
    });
    
    element.addEventListener('drop', function(e) {
        e.preventDefault();
        this.style.borderColor = '#ddd';
        this.style.backgroundColor = 'transparent';
        
        const files = Array.from(e.dataTransfer.files).filter(file => 
            file.name.endsWith('.xlsx') || file.name.endsWith('.xls')
        );
        
        if (files.length > 0) {
            const panel = this.id.replace('-upload', '');
            addFilesToList(panel, files);
        }
    });
}

// 输出设置切换
function toggleOutputSettings(mode) {
    const folderElement = document.getElementById('delete-output-folder');
    const suffixElement = document.getElementById('delete-file-suffix');
    
    if (mode === 'newfile') {
        folderElement.classList.remove('hidden');
        suffixElement.classList.remove('hidden');
    } else {
        folderElement.classList.add('hidden');
        suffixElement.classList.add('hidden');
    }
}

// 选择输出文件夹
function selectOutputFolder(panel) {
    // 改进的文件夹选择功能
    const input = document.createElement('input');
    input.type = 'file';
    input.webkitdirectory = true;
    input.directory = true;
    input.multiple = false;
    input.onchange = function(e) {
        if (e.target.files.length > 0) {
            // 获取完整的文件夹路径
            const fullPath = e.target.files[0].webkitRelativePath;
            const folderPath = fullPath.substring(0, fullPath.lastIndexOf('/')) || e.target.files[0].webkitRelativePath.split('/')[0];
            const pathElement = document.getElementById(`${panel}-output-path`);
            if (pathElement) {
                pathElement.value = folderPath;
            }
            addLog(panel, `已选择输出文件夹: ${folderPath}`);
        }
    };
    input.click();
}

// 执行功能
function executeDelete() {
    console.log('executeDelete function called'); // 调试信息
    
    if (fileLists.delete.length === 0) {
        alert('请先选择文件');
        addLog('delete', '错误: 未选择任何文件');
        return;
    }
    
    // 获取删除设置
    const deleteFirstRows = document.getElementById('delete-first-rows').checked;
    const firstRowsCount = parseInt(document.getElementById('first-rows-count').value) || 0;
    const deleteLastRows = document.getElementById('delete-last-rows').checked;
    const lastRowsCount = parseInt(document.getElementById('last-rows-count').value) || 0;
    const deleteCustomRows = document.getElementById('delete-custom-rows').checked;
    const customRowsInput = document.getElementById('custom-rows-input').value;
    
    const deleteFirstCols = document.getElementById('delete-first-cols').checked;
    const firstColsCount = parseInt(document.getElementById('first-cols-count').value) || 0;
    const deleteCustomCols = document.getElementById('delete-custom-cols').checked;
    const customColsInput = document.getElementById('custom-cols-input').value;
    
    const sheetName = document.getElementById('delete-sheet-select').value;
    const outputModeRadio = document.querySelector('input[name="delete-output-mode"]:checked');
    const outputMode = outputModeRadio ? outputModeRadio.value : 'overwrite';
    
    // 验证是否有删除操作被选中
    const hasDeleteOperation = deleteFirstRows || deleteLastRows || deleteCustomRows || 
                              deleteFirstCols || deleteCustomCols;
    
    if (!hasDeleteOperation) {
        alert('请至少选择一种删除操作');
        addLog('delete', '错误: 未选择任何删除操作');
        return;
    }
    
    addLog('delete', `开始处理删除操作...`);
    addLog('delete', `文件数量: ${fileLists.delete.length}`);
    addLog('delete', `设置: 首${firstRowsCount}行, 尾${lastRowsCount}行, 自定义行:${customRowsInput || '无'}`);
    addLog('delete', `列设置: 首${firstColsCount}列, 自定义列:${customColsInput || '无'}`);
    addLog('delete', `输出模式: ${outputMode}`);
    
    // 构建处理请求
    const formData = new FormData();
    fileLists.delete.forEach((file, index) => {
        formData.append(`files`, file);
        console.log(`添加文件 ${index}: ${file.name}`); // 调试信息
    });
    
    formData.append('sheetName', sheetName);
    formData.append('deleteFirstRows', deleteFirstRows);
    formData.append('firstRowsCount', firstRowsCount);
    formData.append('deleteLastRows', deleteLastRows);
    formData.append('lastRowsCount', lastRowsCount);
    formData.append('deleteCustomRows', deleteCustomRows);
    formData.append('customRowsInput', customRowsInput);
    formData.append('deleteFirstCols', deleteFirstCols);
    formData.append('firstColsCount', firstColsCount);
    formData.append('deleteCustomCols', deleteCustomCols);
    formData.append('customColsInput', customColsInput);
    formData.append('outputMode', outputMode);
    
    addLog('delete', '正在发送请求到服务器...');
    
    // 发送到后端处理
    fetch('/api/delete-rows', {
        method: 'POST',
        body: formData
    })
    .then(response => {
        console.log('Response status:', response.status); // 调试信息
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }
        return response.json();
    })
    .then(data => {
        console.log('Response data:', data); // 调试信息
        if (data.status === 'success') {
            addLog('delete', `✅ 处理完成: ${data.processed}/${data.total} 个文件`);
            addLog('delete', `📁 结果文件已生成在: ${data.outputDir || '原文件位置'}`);
            alert(`处理完成！\n成功处理 ${data.processed} 个文件`);
        } else {
            throw new Error(data.message || '未知错误');
        }
    })
    .catch(error => {
        console.error('处理失败:', error); // 调试信息
        addLog('delete', `❌ 处理失败: ${error.message}`);
        alert(`处理失败: ${error.message}\n请检查控制台获取详细信息`);
    });
}

function executeMerge() {
    if (fileLists.merge.length < 2) {
        alert('请至少选择两个文件进行合并');
        return;
    }
    
    addLog('merge', '开始合并文件...');
    // 模拟处理过程
    setTimeout(() => {
        addLog('merge', '文件合并完成！');
        alert('文件合并完成');
    }, 3000);
}

function executeJoin() {
    if (fileLists.join.length < 2) {
        alert('请至少选择两个表格进行连接');
        return;
    }
    
    const outputFields = document.getElementById('join-output-fields').value.trim();
    if (!outputFields) {
        alert('请输入输出字段');
        return;
    }
    
    addLog('join', `开始连接操作，输出字段: ${outputFields}`);
    // 模拟处理过程
    setTimeout(() => {
        addLog('join', '表格连接完成！');
        alert('表格连接完成');
    }, 2500);
}

// 添加日志
function addLog(panel, message) {
    const logElement = document.getElementById(`${panel}-log`);
    if (logElement) {
        const timestamp = new Date().toLocaleTimeString();
        logElement.value += `[${timestamp}] ${message}\n`;
        logElement.scrollTop = logElement.scrollHeight;
    }
}

// 复制日志
function copyLog(panel) {
    const logElement = document.getElementById(`${panel}-log`);
    if (logElement) {
        logElement.select();
        document.execCommand('copy');
        addLog(panel, '日志已复制到剪贴板');
    }
}

// 连接规则功能
function addJoinRule() {
    const tableBody = document.querySelector('#join-rules-table tbody');
    const row = document.createElement('tr');
    row.innerHTML = `
        <td><select><option>表1</option><option>表2</option></select></td>
        <td><select><option>Sheet1</option></select></td>
        <td><input type="text" placeholder="列名"></td>
        <td>
            <select>
                <option>INNER JOIN</option>
                <option>LEFT JOIN</option>
                <option>RIGHT JOIN</option>
                <option>FULL OUTER JOIN</option>
            </select>
        </td>
        <td><select><option>表1</option><option>表2</option></select></td>
        <td><select><option>Sheet1</option></select></td>
        <td><input type="text" placeholder="列名"></td>
        <td><button class="btn-small btn-danger" onclick="removeRule(this)">×</button></td>
    `;
    tableBody.appendChild(row);
    addLog('join', '已添加连接规则');
}

function removeRule(button) {
    button.closest('tr').remove();
    addLog('join', '已删除连接规则');
}

function addTable(panel) {
    selectFiles(panel);
}

// 获取Sheet信息
function getSheetInfo(panel, file) {
    // 发送请求到后端获取真实的sheet信息
    const formData = new FormData();
    formData.append('file', file);
    
    fetch('/api/sheet-info', {
        method: 'POST',
        body: formData
    })
    .then(response => response.json())
    .then(data => {
        if (data.status === 'success') {
            updateSheetDropdown(panel, data.sheets);
            addLog(panel, `已获取文件 ${file.name} 的Sheet信息: ${data.sheets.join(', ')}`);
        } else {
            throw new Error(data.message);
        }
    })
    .catch(error => {
        console.error('获取Sheet信息失败:', error);
        // 失败时使用默认值
        const defaultSheets = ['Sheet1', 'Sheet2', 'Sheet3'];
        updateSheetDropdown(panel, defaultSheets);
        addLog(panel, `获取Sheet信息失败，使用默认值: ${defaultSheets.join(', ')}`);
    });
}

// 更新Sheet下拉框
function updateSheetDropdown(panel, sheetNames) {
    const sheetSelect = document.getElementById(`${panel}-sheet-select`);
    if (sheetSelect) {
        sheetSelect.innerHTML = '';
        sheetNames.forEach(sheet => {
            const option = document.createElement('option');
            option.value = sheet;
            option.textContent = sheet;
            sheetSelect.appendChild(option);
        });
    }
}

// 退出程序
function exitProgram() {
    if (confirm('确定要退出程序吗？这将关闭所有相关进程。')) {
        // 发送退出请求到后端
        fetch('/api/exit', {
            method: 'POST'
        }).then(() => {
            addLog(currentPanel, '正在退出程序...');
            // 延迟关闭页面，给后端处理时间
            setTimeout(() => {
                window.close();
            }, 1000);
        }).catch(error => {
            console.error('退出请求失败:', error);
            // 如果API调用失败，直接关闭
            window.close();
        });
    }
}

// 格式化文件大小
function formatFileSize(bytes) {
    if (bytes === 0) return '0 Bytes';
    const k = 1024;
    const sizes = ['Bytes', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

// 键盘快捷键
document.addEventListener('keydown', function(e) {
    // Ctrl+1, Ctrl+2, Ctrl+3 切换面板
    if (e.ctrlKey) {
        switch(e.key) {
            case '1':
                e.preventDefault();
                switchPanel('delete');
                break;
            case '2':
                e.preventDefault();
                switchPanel('merge');
                break;
            case '3':
                e.preventDefault();
                switchPanel('join');
                break;
        }
    }
});