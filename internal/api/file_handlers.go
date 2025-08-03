package api

import (
	"net/http"
	"time"

	"file-ranking/internal/logger"
	"file-ranking/internal/storage"

	"github.com/gin-gonic/gin"
)

type FileHandler struct {
	store *storage.FileStore
}

func NewFileHandler(store *storage.FileStore) *FileHandler {
	return &FileHandler{
		store: store,
	}
}

func (h *FileHandler) UploadFile(c *gin.Context) {
	log := logger.GetInstance()
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		log.Error("❌ 上传请求错误: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "获取文件失败: " + err.Error(),
		})
		return
	}
	defer file.Close()

	log.Info("📤 收到上传请求: %s (大小: %d bytes)", header.Filename, header.Size)

	if header.Size > 10*1024*1024 { // 10MB限制
		log.Warn("⚠️ 文件过大: %s (%d bytes)", header.Filename, header.Size)
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "文件大小不能超过10MB",
		})
		return
	}

	fileData, err := h.store.UploadFile(header.Filename, file)
	if err != nil {
		log.Error("❌ 上传失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "上传失败: " + err.Error(),
		})
		return
	}

	log.Info("✅ 上传成功: %s (ID: %s)", fileData.Name, fileData.ID)
	c.JSON(http.StatusCreated, gin.H{
		"status":  "success",
		"data":    fileData,
		"message": "文件上传成功",
	})
}

func (h *FileHandler) GetRanking(c *gin.Context) {
	ranking := h.store.GetRanking()
	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"data":    ranking,
		"message": "获取排行榜成功",
	})
}

func (h *FileHandler) ClickFile(c *gin.Context) {
	log := logger.GetInstance()
	fileID := c.Param("id")
	if fileID == "" {
		log.Warn("⚠️ 点击请求缺少文件ID")
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "文件ID不能为空",
		})
		return
	}

	log.Info("👆 收到点击请求: %s", fileID)

	if err := h.store.IncrementClick(fileID); err != nil {
		log.Error("❌ 点击失败: %v", err)
		c.JSON(http.StatusNotFound, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	file, _ := h.store.GetFile(fileID)
	log.Info("✅ 点击成功: %s (当前点击: %d)", file.Name, file.Clicks)
	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"data":    file,
		"message": "点击成功",
	})
}

func (h *FileHandler) DownloadFile(c *gin.Context) {
	fileID := c.Param("id")
	if fileID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "文件ID不能为空",
		})
		return
	}

	file, exists := h.store.GetFile(fileID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"status":  "error",
			"message": "文件不存在",
		})
		return
	}

	c.FileAttachment(file.Path, file.Name)
}

func (h *FileHandler) RemoveFile(c *gin.Context) {
	fileID := c.Param("id")
	if fileID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "文件ID不能为空",
		})
		return
	}

	if err := h.store.RemoveFile(fileID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "文件删除成功",
	})
}

func (h *FileHandler) GetAllFiles(c *gin.Context) {
	files := h.store.GetAllFiles()
	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"data":    files,
		"message": "获取所有文件成功",
	})
}

// 高性能实时更新监控
func (h *FileHandler) StartRealTimeUpdater(hub *WebSocketHub) {
	ticker := time.NewTicker(100 * time.Millisecond) // 100ms检查一次
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if h.store.CacheInvalid() {
				ranking := h.store.GetRanking()
				if len(ranking) > 0 {
					hub.BroadcastRanking(ranking)
				}
			}
		default: // 添加default防止忙等待
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func (h *FileHandler) GetFile(c *gin.Context) {
	fileID := c.Param("id")
	if fileID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "文件ID不能为空",
		})
		return
	}

	file, exists := h.store.GetFile(fileID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"status":  "error",
			"message": "文件不存在",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"data":    file,
		"message": "获取文件成功",
	})
}

func (h *FileHandler) RenameFile(c *gin.Context) {
	log := logger.GetInstance()
	fileID := c.Param("id")
	if fileID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "文件ID不能为空",
		})
		return
	}

	var req struct {
		NewName string `json:"new_name" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "参数错误: " + err.Error(),
		})
		return
	}

	log.Info("✏️ 收到重命名请求: %s → %s", fileID, req.NewName)

	if err := h.store.RenameFile(fileID, req.NewName); err != nil {
		log.Error("❌ 重命名失败: %v", err)
		c.JSON(http.StatusNotFound, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	file, _ := h.store.GetFile(fileID)
	log.Info("✅ 重命名成功: %s (新名称: %s)", fileID, req.NewName)
	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"data":    file,
		"message": "重命名成功",
	})
}

func (h *FileHandler) GetFileContent(c *gin.Context) {
	log := logger.GetInstance()
	fileID := c.Param("id")
	if fileID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "文件ID不能为空",
		})
		return
	}

	log.Info("📖 获取文件内容: %s", fileID)

	content, err := h.store.GetFileContent(fileID)
	if err != nil {
		log.Error("❌ 获取内容失败: %v", err)
		c.JSON(http.StatusNotFound, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	log.Info("✅ 获取内容成功: %s", fileID)
	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"content": content,
		},
		"message": "获取内容成功",
	})
}

func (h *FileHandler) UpdateFileContent(c *gin.Context) {
	log := logger.GetInstance()
	fileID := c.Param("id")
	if fileID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "文件ID不能为空",
		})
		return
	}

	var req struct {
		Content string `json:"content"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "参数错误: " + err.Error(),
		})
		return
	}

	log.Info("📝 收到更新文件内容请求: %s", fileID)

	if err := h.store.UpdateFileContent(fileID, req.Content); err != nil {
		log.Error("❌ 更新文件内容失败: %v", err)
		c.JSON(http.StatusNotFound, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	file, _ := h.store.GetFile(fileID)
	log.Info("✅ 更新文件内容成功: %s (ID: %s, 点击数: %d)", file.Name, fileID, file.Clicks)
	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"data":    file,
		"message": "文件内容更新成功",
	})
}

func (h *FileHandler) CreateFile(c *gin.Context) {
	log := logger.GetInstance()

	var req struct {
		Name    string `json:"name" binding:"required"`
		Content string `json:"content"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "参数错误: " + err.Error(),
		})
		return
	}

	// 验证文件名
	if req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "文件名不能为空",
		})
		return
	}

	log.Info("📝 收到创建文件请求: %s", req.Name)

	file, err := h.store.CreateFile(req.Name, req.Content)
	if err != nil {
		log.Error("❌ 创建文件失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "创建文件失败: " + err.Error(),
		})
		return
	}

	log.Info("✅ 创建文件成功: %s (ID: %s)", file.Name, file.ID)
	c.JSON(http.StatusCreated, gin.H{
		"status":  "success",
		"data":    file,
		"message": "文件创建成功",
	})
}

func (h *FileHandler) BulkClick(c *gin.Context) {
	var req struct {
		FileID string `json:"file_id" binding:"required"`
		Count  int    `json:"count" binding:"required,min=1,max=1000"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "参数错误: " + err.Error(),
		})
		return
	}

	for i := 0; i < req.Count; i++ {
		if err := h.store.IncrementClick(req.FileID); err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"status":  "error",
				"message": err.Error(),
			})
			return
		}
	}

	file, _ := h.store.GetFile(req.FileID)
	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"file_id": req.FileID,
			"clicks":  file.Clicks,
			"added":   req.Count,
		},
		"message": "批量点击成功",
	})
}
