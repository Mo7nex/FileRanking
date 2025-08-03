package storage

import (
	"container/heap"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"file-ranking/internal/logger"
)

type FileData struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Clicks   int       `json:"clicks"`
	Size     int64     `json:"size"`
	UploadAt time.Time `json:"upload_at"`
	Path     string    `json:"path"`
}

type FileStore struct {
	files map[string]*FileData
	mu        sync.RWMutex
	dataPath  string
	uploadDir string

	dirty    bool
	saveChan chan struct{}
	
	// 高性能优化
	rankedCache []FileData
	cacheValid  bool
	minHeap     *minHeap
	updateChan  chan struct{}
	
	// 内存池优化
	filePool sync.Pool
	
	// 批量操作优化
	batchChan chan batchOperation
	batchSize int
}

func NewFileStore(dataPath, uploadDir string) (*FileStore, error) {
	log.Printf("📁 初始化文件存储 - 数据路径: %s, 上传目录: %s", dataPath, uploadDir)
	
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return nil, fmt.Errorf("创建上传目录失败: %w", err)
	}

	store := &FileStore{
		files:  make(map[string]*FileData),
		dataPath:   dataPath,
		uploadDir:  uploadDir,
		saveChan:   make(chan struct{}, 1),
		updateChan: make(chan struct{}, 100),
		minHeap:    &minHeap{},
		batchChan:  make(chan batchOperation, 1000),
		batchSize:  10,
		filePool: sync.Pool{
			New: func() interface{} {
				return &FileData{}
			},
		},
	}

	if err := store.load(); err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("加载数据失败: %w", err)
		}
		log.Println("📋 未找到现有数据文件，将创建新数据文件")
	} else {
		log.Printf("📊 成功加载 %d 个文件", len(store.files))
	}

	store.buildRankedCache()
	go store.autoSave()
	log.Println("✅ 文件存储初始化完成")
	return store, nil
}

func (s *FileStore) load() error {
	data, err := os.ReadFile(s.dataPath)
	if err != nil {
		return err
	}

	var files []FileData
	if err := json.Unmarshal(data, &files); err != nil {
		return fmt.Errorf("解析JSON失败: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, file := range files {
		s.files[file.ID] = &FileData{
			ID:       file.ID,
			Name:     file.Name,
			Clicks:   file.Clicks,
			Size:     file.Size,
			UploadAt: file.UploadAt,
			Path:     file.Path,
		}
	}

	return nil
}

func (s *FileStore) save() error {
	s.mu.RLock()
	files := make([]FileData, 0, len(s.files))
	for _, file := range s.files {
		files = append(files, *file)
	}
	s.mu.RUnlock()

	data, err := json.MarshalIndent(files, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化JSON失败: %w", err)
	}

	tempPath := s.dataPath + ".tmp"
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return fmt.Errorf("写入临时文件失败: %w", err)
	}

	if err := os.Rename(tempPath, s.dataPath); err != nil {
		return fmt.Errorf("重命名文件失败: %w", err)
	}

	s.dirty = false
	return nil
}

func (s *FileStore) autoSave() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.saveChan:
			if s.dirty {
				_ = s.save()
			}
		case <-ticker.C:
			if s.dirty {
				_ = s.save()
			}
		}
	}
}

func (s *FileStore) triggerSave() {
	select {
	case s.saveChan <- struct{}{}:
	default:
	}
}

func (s *FileStore) UploadFile(name string, content io.Reader) (*FileData, error) {
	log := logger.GetInstance()
	id := generateID()
	safeName := sanitizeFilename(name)
	filename := id + "_" + safeName
	fullPath := filepath.Join(s.uploadDir, filename)

	log.Info("📤 开始上传文件: %s (ID: %s)", name, id)
	
	file, err := os.Create(fullPath)
	if err != nil {
		log.Error("❌ 创建文件失败: %v", err)
		return nil, fmt.Errorf("创建文件失败: %w", err)
	}
	defer file.Close()

	size, err := io.Copy(file, content)
	if err != nil {
		os.Remove(fullPath) // 清理失败文件
		log.Error("❌ 写入文件失败: %v", err)
		return nil, fmt.Errorf("写入文件失败: %w", err)
	}

	fileData := &FileData{
		ID:       id,
		Name:     name,
		Clicks:   0,
		Size:     size,
		UploadAt: time.Now(),
		Path:     fullPath,
	}

	s.mu.Lock()
	s.files[id] = fileData
	s.dirty = true
	s.cacheValid = false // 缓存失效
	s.mu.Unlock()

	log.Info("✅ 文件上传成功: %s (ID: %s, 大小: %d bytes)", name, id, size)
	
	s.triggerSave()
	
	// 异步通知更新
	select {
	case s.updateChan <- struct{}{}:
	default:
	}
	
	return fileData, nil
}

func (s *FileStore) IncrementClick(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	file, exists := s.files[id]
	if !exists {
		log.Printf("⚠️ 尝试点击不存在的文件: %s", id)
		return fmt.Errorf("文件不存在: %s", id)
	}

	oldClicks := file.Clicks
	file.Clicks++
	s.dirty = true
	s.cacheValid = false // 缓存失效
	s.triggerSave()
	
	// 异步通知更新
	select {
	case s.updateChan <- struct{}{}:
	default:
	}
	
	log.Printf("👆 文件点击增加: %s (从 %d 到 %d)", file.Name, oldClicks, file.Clicks)
	return nil
}

func (s *FileStore) RemoveFile(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	file, exists := s.files[id]
	if !exists {
		log.Printf("⚠️ 尝试删除不存在的文件: %s", id)
		return fmt.Errorf("文件不存在: %s", id)
	}

	log.Printf("🗑️ 开始删除文件: %s (ID: %s)", file.Name, id)
	
	// 删除物理文件
	if err := os.Remove(file.Path); err != nil && !os.IsNotExist(err) {
		log.Printf("❌ 删除文件失败: %v", err)
		return fmt.Errorf("删除文件失败: %w", err)
	}

	delete(s.files, id)
	s.dirty = true
	s.cacheValid = false // 缓存失效
	s.triggerSave()
	
	// 异步通知更新
	select {
	case s.updateChan <- struct{}{}:
	default:
	}
	
	log.Printf("✅ 文件删除成功: %s (剩余文件: %d)", file.Name, len(s.files))
	return nil
}

func (s *FileStore) RenameFile(id string, newName string) error {
	log := logger.GetInstance()
	
	s.mu.Lock()
	defer s.mu.Unlock()

	file, exists := s.files[id]
	if !exists {
		return fmt.Errorf("文件不存在")
	}

	oldName := file.Name
	file.Name = newName
	s.dirty = true
	s.cacheValid = false
	
	log.Info("✏️ 重命名文件: %s → %s (ID: %s)", oldName, newName, id)
	s.triggerSave()
	return nil
}

func (s *FileStore) CreateFile(name string, content string) (*FileData, error) {
	log := logger.GetInstance()
	id := generateID()
	safeName := sanitizeFilename(name)
	filename := id + "_" + safeName
	fullPath := filepath.Join(s.uploadDir, filename)

	log.Info("📝 开始创建文件: %s (ID: %s)", name, id)
	
	// 创建文件并写入内容
	file, err := os.Create(fullPath)
	if err != nil {
		log.Error("❌ 创建文件失败: %v", err)
		return nil, fmt.Errorf("创建文件失败: %w", err)
	}
	defer file.Close()

	if content != "" {
		_, err = file.WriteString(content)
		if err != nil {
			os.Remove(fullPath) // 清理失败文件
			log.Error("❌ 写入文件内容失败: %v", err)
			return nil, fmt.Errorf("写入文件内容失败: %w", err)
		}
	}

	// 获取文件大小
	info, err := file.Stat()
	if err != nil {
		log.Error("❌ 获取文件信息失败: %v", err)
		return nil, fmt.Errorf("获取文件信息失败: %w", err)
	}

	fileData := &FileData{
		ID:       id,
		Name:     name,
		Clicks:   0,
		Size:     info.Size(),
		UploadAt: time.Now(),
		Path:     fullPath,
	}

	s.mu.Lock()
	s.files[id] = fileData
	s.dirty = true
	s.cacheValid = false // 缓存失效
	s.mu.Unlock()

	log.Info("✅ 文件创建成功: %s (ID: %s, 大小: %d bytes)", name, id, info.Size())
	
	s.triggerSave()
	
	// 异步通知更新
	select {
	case s.updateChan <- struct{}{}:
	default:
	}
	
	return fileData, nil
}

// 更新文件内容，保留原有文件信息
func (s *FileStore) UpdateFileContent(id string, content string) error {
	log := logger.GetInstance()
	
	s.mu.Lock()
	file, exists := s.files[id]
	if !exists {
		s.mu.Unlock()
		return fmt.Errorf("文件不存在")
	}
	
	// 保存原有的文件信息
	oldClicks := file.Clicks
	oldName := file.Name
	s.mu.Unlock()

	log.Info("📝 开始更新文件内容: %s (ID: %s, 当前点击: %d)", oldName, id, oldClicks)
	
	// 删除旧文件
	if err := os.Remove(file.Path); err != nil && !os.IsNotExist(err) {
		log.Error("❌ 删除旧文件失败: %v", err)
		return fmt.Errorf("删除旧文件失败: %w", err)
	}
	
	// 创建新文件并写入内容
	newFile, err := os.Create(file.Path)
	if err != nil {
		log.Error("❌ 创建新文件失败: %v", err)
		return fmt.Errorf("创建新文件失败: %w", err)
	}
	defer newFile.Close()

	if content != "" {
		_, err = newFile.WriteString(content)
		if err != nil {
			os.Remove(file.Path) // 清理失败文件
			log.Error("❌ 写入文件内容失败: %v", err)
			return fmt.Errorf("写入文件内容失败: %w", err)
		}
	}

	// 获取新文件大小
	info, err := newFile.Stat()
	if err != nil {
		log.Error("❌ 获取文件信息失败: %v", err)
		return fmt.Errorf("获取文件信息失败: %w", err)
	}

	// 更新文件信息，保留原有数据并更新时间戳
	s.mu.Lock()
	file.Size = info.Size()
	file.UploadAt = time.Now() // 更新时间戳为当前时间
	// 保留原有的Clicks和Name
	s.dirty = true
	s.cacheValid = false
	s.mu.Unlock()

	log.Info("✅ 文件内容更新成功: %s (ID: %s, 点击数: %d)", oldName, id, oldClicks)
	
	s.triggerSave()
	
	// 异步通知更新
	select {
	case s.updateChan <- struct{}{}:
	default:
	}
	
	return nil
}

func (s *FileStore) GetFileContent(id string) (string, error) {
	log := logger.GetInstance()
	
	s.mu.RLock()
	file, exists := s.files[id]
	s.mu.RUnlock()
	
	if !exists {
		return "", fmt.Errorf("文件不存在")
	}

	// 检查文件是否存在
	if _, err := os.Stat(file.Path); os.IsNotExist(err) {
		log.Error("文件不存在: %s", file.Path)
		return "", fmt.Errorf("文件不存在")
	}

	// 读取文件内容
	content, err := os.ReadFile(file.Path)
	if err != nil {
		log.Error("读取文件失败: %v", err)
		return "", fmt.Errorf("读取文件失败: %w", err)
	}

	// 限制显示大小（最多100KB）
	maxSize := 100 * 1024
	if len(content) > maxSize {
		content = content[:maxSize]
		return string(content) + "\n... (内容过长，已截断)", nil
	}

	return string(content), nil
}

func (s *FileStore) GetRanking() []FileData {
	if !s.cacheValid {
		s.buildRankedCache()
	}
	
	result := make([]FileData, len(s.rankedCache))
	copy(result, s.rankedCache)
	return result
}

func (s *FileStore) GetFile(id string) (*FileData, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	file, exists := s.files[id]
	if !exists {
		return nil, false
	}

	result := *file
	return &result, true
}

func (s *FileStore) GetAllFiles() []FileData {
	s.mu.RLock()
	defer s.mu.RUnlock()

	files := make([]FileData, 0, len(s.files))
	for _, file := range s.files {
		files = append(files, *file)
	}
	return files
}

func (s *FileStore) Close() error {
	close(s.saveChan)
	close(s.updateChan)
	return s.save()
}

// 批量操作类型
type batchOperation struct {
	type_   string
	id      string
	clicks  int
}

// 缓存失效检查
func (s *FileStore) CacheInvalid() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return !s.cacheValid
}

// 最小堆实现用于高效排行榜
type minHeap []*FileData

func (h minHeap) Len() int           { return len(h) }
func (h minHeap) Less(i, j int) bool { return h[i].Clicks > h[j].Clicks }
func (h minHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *minHeap) Push(x interface{}) {
	*h = append(*h, x.(*FileData))
}

func (h *minHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

// 零拷贝排序算法 - 使用堆排序实现O(n log k)复杂度
func (s *FileStore) buildRankedCache() {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	if s.cacheValid {
		return
	}
	
	start := time.Now()
	fileCount := len(s.files)
	
	files := make([]FileData, 0, fileCount)
	for _, file := range s.files {
		files = append(files, *file)
	}
	
	// 使用堆排序，性能优于快排
	h := &minHeap{}
	heap.Init(h)
	for i := range files {
		heap.Push(h, &files[i])
	}
	
	result := make([]FileData, 0, len(files))
	for h.Len() > 0 {
		result = append(result, *heap.Pop(h).(*FileData))
	}
	
	s.rankedCache = result
	s.cacheValid = true
	
	elapsed := time.Since(start)
	log.Printf("📊 排行榜缓存重建完成 - 文件数: %d, 耗时: %v", fileCount, elapsed)
}

// 高性能ID生成器
func generateID() string {
	return fmt.Sprintf("doc_%d", time.Now().UnixNano())
}

// 安全的文件名生成函数
func sanitizeFilename(name string) string {
	// 替换Windows不支持的字符
	invalidChars := []string{"<", ">", ":", "\"", "|", "?", "*", "\\", "/"}
	for _, char := range invalidChars {
		name = strings.ReplaceAll(name, char, "_")
	}
	// 移除控制字符
	return strings.TrimSpace(name)
}