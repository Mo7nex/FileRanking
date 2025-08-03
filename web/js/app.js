// 文件管理系统 JavaScript

// API基础URL
const API_BASE = '';



// 显示查看内容模态框
function showViewModal(fileId, fileName) {
    console.log('查看按钮点击触发，文件ID:', fileId, '文件名:', fileName);
    
    // 先获取文件内容
    console.log('开始发送API请求到:', `${API_BASE}/api/files/${fileId}/content`);
    
    fetch(`${API_BASE}/api/files/${fileId}/content`)
        .then(response => {
            console.log('API响应状态:', response.status);
            if (!response.ok) {
                throw new Error(`HTTP ${response.status}: ${response.statusText}`);
            }
            return response.json();
        })
        .then(data => {
            console.log('API响应数据:', data);
            if (data.status === 'success') {
                const modalHTML = `
                    <div class="modal" id="viewModal">
                        <div class="modal-content" style="max-width: 800px; width: 95%;">
                            <h2 class="modal-header">查看文件: ${escapeHtml(fileName)}</h2>
                            <div class="form-group">
                                <label class="form-label">文件内容</label>
                                <div class="file-content-view" style="
                                    background: rgba(248, 249, 250, 0.8);
                                    border: 1px solid #dee2e6;
                                    border-radius: 8px;
                                    padding: 15px;
                                    max-height: 500px;
                                    overflow-y: auto;
                                    font-family: monospace;
                                    font-size: 14px;
                                    line-height: 1.5;
                                    white-space: pre-wrap;
                                    word-break: break-all;
                                ">${escapeHtml(data.data.content || '(空文件)')}</div>
                            </div>
                            <div class="form-actions">
                                <button type="button" class="btn-cancel" onclick="closeModal()">关闭</button>
                                <button type="button" class="btn-edit" onclick="showEditModal('${fileId}', '${escapeHtml(fileName)}'); closeModal();">编辑</button>
                            </div>
                        </div>
                    </div>
                `;
                
                document.body.insertAdjacentHTML('beforeend', modalHTML);
                
                document.getElementById('viewModal').addEventListener('click', function(e) {
                    if (e.target === this) {
                        closeModal();
                    }
                });
            } else {
                showMessage('获取文件内容失败: ' + data.message, 'error');
            }
        })
        .catch(error => {
            console.error('获取文件内容错误:', error);
            showMessage('获取文件内容失败: ' + error.message, 'error');
        });
}



// 初始化应用
document.addEventListener('DOMContentLoaded', function() {
    initializeApp();
});

function initializeApp() {
    setupEventListeners();
    fetchData();
    connectWebSocket();
}

function setupEventListeners() {
    // 文件上传相关事件
    const uploadArea = document.getElementById('uploadArea');
    const fileInput = document.getElementById('fileInput');
    
    if (uploadArea) {
        uploadArea.addEventListener('click', () => fileInput.click());
        uploadArea.addEventListener('dragover', handleDragOver);
        uploadArea.addEventListener('dragleave', handleDragLeave);
        uploadArea.addEventListener('drop', handleDrop);
    }
    
    if (fileInput) {
        fileInput.addEventListener('change', handleFileSelect);
    }
    
    // 创建文件按钮
    const createBtn = document.getElementById('createBtn');
    if (createBtn) {
        createBtn.addEventListener('click', showCreateModal);
    }
}

// 文件上传处理
function handleDragOver(e) {
    e.preventDefault();
    e.currentTarget.classList.add('dragover');
}

function handleDragLeave(e) {
    e.preventDefault();
    e.currentTarget.classList.remove('dragover');
}

function handleDrop(e) {
    e.preventDefault();
    e.currentTarget.classList.remove('dragover');
    const files = Array.from(e.dataTransfer.files);
    files.forEach(uploadFile);
}

function handleFileSelect(e) {
    const files = Array.from(e.target.files);
    files.forEach(uploadFile);
}

function uploadFile(file) {
    const formData = new FormData();
    formData.append('file', file);
    
    fetch(`${API_BASE}/api/files/upload`, {
        method: 'POST',
        body: formData
    })
    .then(response => {
        if (!response.ok && response.status !== 0) {
            throw new Error('服务连接失败');
        }
        return response.json();
    })
    .then(data => {
        if (data.status === 'success') {
            showMessage('文件上传成功');
            fetchData();
        } else {
            showMessage('上传失败: ' + data.message, 'error');
        }
    })
    .catch(error => {
        if (navigator.onLine && !error.message.includes('Failed to fetch')) {
            console.error('上传错误:', error);
            showMessage('上传失败: ' + error.message, 'error');
        }
    });
}

// 数据获取
async function fetchData() {
    try {
        const response = await fetch(`${API_BASE}/api/files`);
        if (!response.ok) {
            if (response.status === 0 || response.status === 502) {
                return;
            }
        }
        const data = await response.json();
        
        if (data.status === 'success') {
            updateStats(data.data);
            renderAllFiles(data.data);
            renderRankingList(data.data);
        }
    } catch (error) {
        if (error.name !== 'AbortError') {
            if (navigator.onLine) {
                console.log('等待服务启动...');
            }
        }
    }
}

// 数字动画效果
function animateNumber(element, start, end, duration = 1000) {
    if (start === end) return;
    
    const range = end - start;
    const increment = range / (duration / 16);
    let current = start;
    
    const timer = setInterval(() => {
        current += increment;
        if ((increment > 0 && current >= end) || (increment < 0 && current <= end)) {
            current = end;
            clearInterval(timer);
        }
        
        if (element.id === 'totalSize') {
            element.textContent = formatFileSize(Math.round(current));
        } else {
            element.textContent = Math.round(current);
        }
    }, 16);
}

// 更新统计信息
function updateStats(files) {
    const totalCount = files.length;
    const totalClicks = files.reduce((sum, file) => sum + file.clicks, 0);
    const totalSize = files.reduce((sum, file) => sum + file.size, 0);
    
    const countEl = document.getElementById('totalCount');
    const clicksEl = document.getElementById('totalClicks');
    const sizeEl = document.getElementById('totalSize');
    
    const currentCount = parseInt(countEl.textContent) || 0;
    const currentClicks = parseInt(clicksEl.textContent) || 0;
    const currentSize = parseFloat(sizeEl.textContent) || 0;
    
    animateNumber(countEl, currentCount, totalCount);
    animateNumber(clicksEl, currentClicks, totalClicks);
    animateNumber(sizeEl, currentSize, totalSize);
    
    const statItems = document.querySelectorAll('.stat-item');
    statItems.forEach(item => {
        item.style.transform = 'scale(1.05)';
        setTimeout(() => {
            item.style.transform = 'scale(1)';
        }, 200);
    });
}

// 渲染所有文件（按创建时间排序）
function renderAllFiles(files) {
    const container = document.getElementById('fileList');
    
    if (!container) return;
    
    if (files.length === 0) {
        container.innerHTML = `
            <div class="empty-state">
                <div class="empty-icon">📁</div>
                <p>暂无文件，请上传或创建新文件</p>
            </div>
        `;
        return;
    }
    
    // 按创建时间排序（最新的在前）
    const sortedFiles = [...files].sort((a, b) => new Date(b.upload_at) - new Date(a.upload_at));
    
    container.innerHTML = sortedFiles.map(file => `
        <div class="file-item" onclick="incrementClick('${file.id}')">
            <div class="file-info">
                <div class="file-name">${escapeHtml(file.name)}</div>
                <div class="file-size">大小: ${formatFileSize(file.size)}</div>
                <div class="file-date">修改时间: ${formatDate(file.upload_at)}</div>
                <div class="file-clicks">点击次数: ${file.clicks}</div>
                <div class="file-actions" onclick="event.stopPropagation()">
                    <button class="btn-view" onclick="showViewModal('${file.id}', '${escapeHtml(file.name)}')" title="查看内容">查看</button>
                    <button class="btn-rename" onclick="showRenameModal('${file.id}', '${escapeHtml(file.name)}')" title="重命名">重命名</button>
                    <button class="btn-edit" onclick="showEditModal('${file.id}', '${escapeHtml(file.name)}')" title="编辑内容">编辑</button>
                    <button class="btn-delete" onclick="deleteFile('${file.id}')" title="删除">删除</button>
                </div>
            </div>
        </div>
    `).join('');
}

// 渲染排行榜（按点击次数排序）
function renderRankingList(files) {
    const container = document.getElementById('rankingList');
    
    if (!container) return;
    
    if (files.length === 0) {
        container.innerHTML = `
            <div class="empty-state">
                <div class="empty-icon">🏆</div>
                <p>暂无排行数据</p>
            </div>
        `;
        return;
    }
    
    // 按点击次数排序
    const rankedFiles = [...files]
        .sort((a, b) => b.clicks - a.clicks)
        .slice(0, 20); // 显示前20名
    
    container.innerHTML = `
        <div class="ranking-table">
            <div class="ranking-header">
                <div class="ranking-col rank">排名</div>
                <div class="ranking-col name">文件名</div>
                <div class="ranking-col clicks">点击数</div>
                <div class="ranking-col size">大小</div>
                <div class="ranking-col date">创建时间</div>
            </div>
            ${rankedFiles.map((file, index) => {
                const rankBadge = index < 3 ? ['🥇', '🥈', '🥉'][index] : (index + 1);
                return `
                    <div class="ranking-row ${index < 3 ? 'top-' + (index + 1) : ''}" onclick="incrementClick('${file.id}')">
                        <div class="ranking-col rank">${rankBadge}</div>
                        <div class="ranking-col name">${escapeHtml(file.name)}</div>
                        <div class="ranking-col clicks">${file.clicks}</div>
                        <div class="ranking-col size">${formatFileSize(file.size)}</div>
                        <div class="ranking-col date">${formatDate(file.upload_at)}</div>
                    </div>
                `;
            }).join('')}
        </div>
    `;
}

// 增加点击次数
function incrementClick(fileId) {
    fetch(`${API_BASE}/api/files/${fileId}/click`, {
        method: 'POST'
    })
    .then(response => response.json())
    .then(data => {
        if (data.status === 'success') {
            fetchData();
        }
    })
    .catch(error => {
        console.error('点击统计错误:', error);
    });
}

// WebSocket连接
function connectWebSocket() {
    const ws = new WebSocket(`ws://localhost:8080/api/ws`);
    
    ws.onopen = function() {
        console.log('WebSocket连接已建立');
    };
    
    ws.onmessage = function(event) {
        const data = JSON.parse(event.data);
        if (data.type === 'update') {
            fetchData();
        }
    };
    
    ws.onclose = function() {
        console.log('WebSocket连接断开');
        // 5秒后重连
        setTimeout(connectWebSocket, 5000);
    };
    
    ws.onerror = function(error) {
        console.error('WebSocket错误:', error);
    };
}

// 创建文件模态框
function showCreateModal() {
    const modalHTML = `
        <div class="modal" id="createModal">
            <div class="modal-content">
                <h2 class="modal-header">创建新文件</h2>
                <form id="createForm">
                    <div class="form-group">
                        <label class="form-label">文件名</label>
                        <input type="text" class="form-input" id="fileName" placeholder="输入文件名" required>
                    </div>
                    <div class="form-group">
                        <label class="form-label">文件内容</label>
                        <textarea class="form-textarea" id="fileContent" placeholder="输入文件内容"></textarea>
                    </div>
                    <div class="form-actions">
                        <button type="button" class="btn-cancel" onclick="closeModal()">取消</button>
                        <button type="submit" class="btn-create">创建</button>
                    </div>
                </form>
            </div>
        </div>
    `;
    
    document.body.insertAdjacentHTML('beforeend', modalHTML);
    
    document.getElementById('createForm').addEventListener('submit', function(e) {
        e.preventDefault();
        createFile();
    });
    
    document.getElementById('createModal').addEventListener('click', function(e) {
        if (e.target === this) {
            closeModal();
        }
    });
}

function createFile() {
    const name = document.getElementById('fileName').value;
    const content = document.getElementById('fileContent').value;
    
    fetch(`${API_BASE}/api/files/create`, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify({ name, content })
    })
    .then(response => {
        if (!response.ok) {
            throw new Error(`HTTP ${response.status}: ${response.statusText}`);
        }
        return response.json();
    })
    .then(data => {
        if (data.status === 'success') {
            showMessage('文件创建成功');
            closeModal();
            fetchData();
        } else {
            showMessage('创建失败: ' + data.message, 'error');
        }
    })
    .catch(error => {
        console.error('创建错误:', error);
        showMessage('创建失败: ' + error.message, 'error');
    });
}

function closeModal() {
    const modal = document.getElementById('createModal') || document.getElementById('renameModal') || document.getElementById('editModal') || document.getElementById('viewModal');
    if (modal) {
        modal.remove();
    }
}

// 显示重命名模态框
function showRenameModal(fileId, currentName) {
    const modalHTML = `
        <div class="modal" id="renameModal">
            <div class="modal-content">
                <h2 class="modal-header">重命名文件</h2>
                <form id="renameForm">
                    <div class="form-group">
                        <label class="form-label">新文件名</label>
                        <input type="text" class="form-input" id="newFileName" value="${currentName}" required>
                    </div>
                    <div class="form-actions">
                        <button type="button" class="btn-cancel" onclick="closeModal()">取消</button>
                        <button type="submit" class="btn-create">重命名</button>
                    </div>
                </form>
            </div>
        </div>
    `;
    
    document.body.insertAdjacentHTML('beforeend', modalHTML);
    
    document.getElementById('renameForm').addEventListener('submit', function(e) {
        e.preventDefault();
        renameFile(fileId);
    });
    
    document.getElementById('renameModal').addEventListener('click', function(e) {
        if (e.target === this) {
            closeModal();
        }
    });
}

// 重命名文件
function renameFile(fileId) {
    const newName = document.getElementById('newFileName').value;
    
    fetch(`${API_BASE}/api/files/${fileId}/rename`, {
        method: 'PUT',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify({ new_name: newName })
    })
    .then(response => {
        if (!response.ok) {
            throw new Error(`HTTP ${response.status}: ${response.statusText}`);
        }
        return response.json();
    })
    .then(data => {
        if (data.status === 'success') {
            showMessage('重命名成功');
            closeModal();
            fetchData();
        } else {
            showMessage('重命名失败: ' + data.message, 'error');
        }
    })
    .catch(error => {
        console.error('重命名错误:', error);
        showMessage('重命名失败: ' + error.message, 'error');
    });
}

// 显示编辑内容模态框
function showEditModal(fileId, fileName) {
    // 先获取文件内容
    fetch(`${API_BASE}/api/files/${fileId}/content`)
        .then(response => {
            if (!response.ok) {
                throw new Error(`HTTP ${response.status}: ${response.statusText}`);
            }
            return response.json();
        })
        .then(data => {
            if (data.status === 'success') {
                const modalHTML = `
                    <div class="modal" id="editModal">
                        <div class="modal-content">
                            <h2 class="modal-header">编辑文件: ${escapeHtml(fileName)}</h2>
                            <form id="editForm">
                                <div class="form-group">
                                    <label class="form-label">文件内容</label>
                                    <textarea class="form-textarea" id="fileContentEdit" placeholder="输入文件内容">${escapeHtml(data.data.content || '')}</textarea>
                                </div>
                                <div class="form-actions">
                                    <button type="button" class="btn-cancel" onclick="closeModal()">取消</button>
                                    <button type="submit" class="btn-create">保存</button>
                                </div>
                            </form>
                        </div>
                    </div>
                `;
                
                document.body.insertAdjacentHTML('beforeend', modalHTML);
                
                document.getElementById('editForm').addEventListener('submit', function(e) {
                    e.preventDefault();
                    editFile(fileId, fileName);
                });
                
                document.getElementById('editModal').addEventListener('click', function(e) {
                    if (e.target === this) {
                        closeModal();
                    }
                });
            } else {
                showMessage('获取文件内容失败: ' + data.message, 'error');
            }
        })
        .catch(error => {
            console.error('获取文件内容错误:', error);
            showMessage('获取文件内容失败: ' + error.message, 'error');
        });
}

// 编辑文件内容
function editFile(fileId, fileName) {
    const content = document.getElementById('fileContentEdit').value;
    
    // 使用新的更新API，保留原有文件信息（包括点击次数）
    fetch(`${API_BASE}/api/files/${fileId}/content/edit`, {
        method: 'PUT',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify({ content: content })
    })
    .then(response => {
        if (!response.ok) {
            throw new Error(`HTTP ${response.status}: ${response.statusText}`);
        }
        return response.json();
    })
    .then(data => {
        if (data.status === 'success') {
            showMessage('文件内容更新成功');
            closeModal();
            fetchData();
        } else {
            showMessage('更新失败: ' + data.message, 'error');
        }
    })
    .catch(error => {
        console.error('编辑错误:', error);
        showMessage('编辑失败: ' + error.message, 'error');
    });
}

// 删除文件
function deleteFile(fileId) {
    if (confirm('确定要删除这个文件吗？此操作不可恢复。')) {
        fetch(`${API_BASE}/api/files/${fileId}`, {
            method: 'DELETE'
        })
        .then(response => response.json())
        .then(data => {
            if (data.status === 'success') {
                showMessage('文件删除成功');
                fetchData();
            } else {
                showMessage('删除失败: ' + data.message, 'error');
            }
        })
        .catch(error => {
            console.error('删除错误:', error);
            showMessage('删除失败: ' + error.message, 'error');
        });
    }
}

// 格式化文件大小
function formatFileSize(bytes) {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
}

// 格式化日期
function formatDate(dateString) {
    const date = new Date(dateString);
    const now = new Date();
    const diff = now - date;
    
    if (diff < 60000) return '刚刚';
    if (diff < 3600000) return Math.floor(diff / 60000) + '分钟前';
    if (diff < 86400000) return Math.floor(diff / 3600000) + '小时前';
    if (diff < 604800000) return Math.floor(diff / 86400000) + '天前';
    
    return date.toLocaleDateString('zh-CN', {
        year: 'numeric',
        month: '2-digit',
        day: '2-digit'
    });
}

// 转义HTML
function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

// 显示消息
function showMessage(message, type = 'success') {
    const messageDiv = document.createElement('div');
    messageDiv.className = `message ${type}`;
    messageDiv.textContent = message;
    
    document.body.appendChild(messageDiv);
    
    setTimeout(() => {
        messageDiv.classList.add('show');
    }, 100);
    
    setTimeout(() => {
        messageDiv.classList.remove('show');
        setTimeout(() => {
            messageDiv.remove();
        }, 300);
    }, 3000);
}

// 消息样式
const style = document.createElement('style');
style.textContent = `
    .message {
        position: fixed;
        top: 20px;
        right: 20px;
        padding: 15px 20px;
        border-radius: 10px;
        color: white;
        font-weight: bold;
        z-index: 1000;
        transform: translateX(100%);
        transition: transform 0.3s ease;
        backdrop-filter: blur(10px);
    }
    
    .message.show {
        transform: translateX(0);
    }
    
    .message.success {
        background: rgba(76, 175, 80, 0.9);
    }
    
    .message.error {
        background: rgba(244, 67, 54, 0.9);
    }
    
    .modal {
        position: fixed;
        top: 0;
        left: 0;
        width: 100%;
        height: 100%;
        background: rgba(0, 0, 0, 0.5);
        backdrop-filter: blur(5px);
        display: flex;
        align-items: center;
        justify-content: center;
        z-index: 1000;
    }
    
    .modal-content {
        background: rgba(255, 255, 255, 0.95);
        backdrop-filter: blur(10px);
        padding: 30px;
        border-radius: 20px;
        width: 90%;
        max-width: 500px;
        box-shadow: 0 10px 30px rgba(0, 0, 0, 0.3);
    }
    
    .modal-header {
        margin-top: 0;
        color: #333;
        margin-bottom: 20px;
        font-size: 1.5rem;
    }
    
    .form-group {
        margin: 20px 0;
    }
    
    .form-label {
        display: block;
        margin-bottom: 5px;
        font-weight: bold;
        color: #333;
    }
    
    .form-input, .form-textarea {
        width: 100%;
        padding: 12px;
        border: 1px solid #ddd;
        border-radius: 8px;
        font-size: 16px;
        background: rgba(255, 255, 255, 0.8);
    }
    
    .form-textarea {
        min-height: 200px;
        font-family: monospace;
        resize: vertical;
    }
    
    .form-actions {
        display: flex;
        gap: 10px;
        justify-content: flex-end;
        margin-top: 20px;
    }
    
    .btn-view {
        background: linear-gradient(135deg, #4facfe 0%, #00f2fe 100%);
        color: white;
        border: none;
        padding: 8px 16px;
        border-radius: 8px;
        cursor: pointer;
        font-size: 14px;
        transition: all 0.3s ease;
        margin: 0 5px;
    }

    .btn-view:hover {
        transform: translateY(-2px);
        box-shadow: 0 4px 12px rgba(79, 172, 254, 0.3);
    }

    .btn-edit {
        background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
        color: white;
        border: none;
        padding: 8px 16px;
        border-radius: 8px;
        cursor: pointer;
        font-size: 14px;
        transition: all 0.3s ease;
        margin: 0 5px;
    }

    .btn-edit:hover {
        transform: translateY(-2px);
        box-shadow: 0 4px 12px rgba(102, 126, 234, 0.3);
    }

    .btn-delete {
        background: linear-gradient(135deg, #f093fb 0%, #f5576c 100%);
        color: white;
        border: none;
        padding: 8px 16px;
        border-radius: 8px;
        cursor: pointer;
        font-size: 14px;
        transition: all 0.3s ease;
        margin: 0 5px;
    }

    .btn-delete:hover {
        transform: translateY(-2px);
        box-shadow: 0 4px 12px rgba(245, 87, 108, 0.3);
    }

    .btn-cancel, .btn-create {
        padding: 12px 24px;
        border: none;
        border-radius: 8px;
        cursor: pointer;
        font-size: 16px;
        transition: all 0.3s ease;
    }

    .btn-cancel {
        background: rgba(108, 117, 125, 0.8);
        color: white;
    }

    .btn-create {
        background: rgba(76, 175, 80, 0.8);
        color: white;
    }

    .btn-cancel:hover, .btn-create:hover {
        transform: translateY(-1px);
    }
`;
document.head.appendChild(style);

// 将函数绑定到全局作用域
window.showViewModal = showViewModal;
window.showRenameModal = showRenameModal;
window.showEditModal = showEditModal;
window.deleteFile = deleteFile;
window.closeModal = closeModal;
window.showCreateModal = showCreateModal;
window.incrementClick = incrementClick;
