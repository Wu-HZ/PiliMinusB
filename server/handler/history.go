package handler

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"piliminusb/bilibili"
	"piliminusb/database"
	"piliminusb/middleware"
	"piliminusb/model"
	"piliminusb/response"
)

// ---------------------------------------------------------------------------
// GET /x/web-interface/history/cursor  — 获取历史列表（游标分页）
// ---------------------------------------------------------------------------

func HistoryList(c *gin.Context) {
	userID := middleware.GetUserID(c)

	typ := c.DefaultQuery("type", "")
	ps, _ := strconv.Atoi(c.DefaultQuery("ps", "20"))
	maxOid, _ := strconv.ParseInt(c.DefaultQuery("max", "0"), 10, 64)
	viewAt, _ := strconv.ParseInt(c.DefaultQuery("view_at", "0"), 10, 64)

	query := database.DB.Where("user_id = ?", userID)

	// Filter by business type if specified and not empty/all
	if typ != "" && typ != "all" {
		query = query.Where("business = ?", typ)
	}

	// Cursor pagination: fetch items older than the cursor
	if viewAt > 0 {
		query = query.Where("view_at < ?", viewAt)
	} else if maxOid > 0 {
		// Fallback: use max (aid) as cursor
		var cursor model.WatchHistory
		if err := database.DB.Where("user_id = ? AND aid = ?", userID, maxOid).
			First(&cursor).Error; err == nil {
			query = query.Where("view_at < ?", cursor.ViewAt)
		}
	}

	var items []model.WatchHistory
	query.Order("view_at DESC").Limit(ps + 1).Find(&items)

	hasMore := len(items) > ps
	if hasMore {
		items = items[:ps]
	}

	list := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		list = append(list, item.ToBiliJSON())
	}

	// Build cursor for next page
	var cursorData map[string]interface{}
	if hasMore && len(items) > 0 {
		last := items[len(items)-1]
		cursorData = map[string]interface{}{
			"max":      last.Aid,
			"view_at":  last.ViewAt,
			"business": typ,
			"ps":       ps,
		}
	} else {
		cursorData = map[string]interface{}{
			"max":      0,
			"view_at":  0,
			"business": typ,
			"ps":       ps,
		}
	}

	response.Success(c, gin.H{
		"cursor": cursorData,
		"list":   list,
		"tab":    []interface{}{},
	})
}

// ---------------------------------------------------------------------------
// GET /x/web-interface/history/search  — 搜索历史记录
// ---------------------------------------------------------------------------

func SearchHistory(c *gin.Context) {
	userID := middleware.GetUserID(c)

	pn, _ := strconv.Atoi(c.DefaultQuery("pn", "1"))
	keyword := c.DefaultQuery("keyword", "")
	business := c.DefaultQuery("business", "all")

	ps := 20
	offset := (pn - 1) * ps

	query := database.DB.Where("user_id = ?", userID)

	if business != "" && business != "all" {
		query = query.Where("business = ?", business)
	}
	if keyword != "" {
		query = query.Where("title LIKE ?", "%"+keyword+"%")
	}

	var items []model.WatchHistory
	query.Order("view_at DESC").Offset(offset).Limit(ps + 1).Find(&items)

	hasMore := len(items) > ps
	if hasMore {
		items = items[:ps]
	}

	list := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		list = append(list, item.ToBiliJSON())
	}

	// searchHistory returns page-based pagination but same data structure
	var cursorData map[string]interface{}
	if hasMore {
		cursorData = map[string]interface{}{
			"max":      0,
			"view_at":  0,
			"business": business,
			"ps":       ps,
		}
	} else {
		cursorData = map[string]interface{}{
			"max":      0,
			"view_at":  0,
			"business": business,
			"ps":       ps,
		}
	}

	response.Success(c, gin.H{
		"cursor":   cursorData,
		"list":     list,
		"has_more": hasMore,
		"page": gin.H{
			"pn":    pn,
			"ps":    ps,
			"total": 0, // not worth a separate count query
		},
	})
}

// ---------------------------------------------------------------------------
// POST /x/v2/history/delete  — 删除单条历史记录
// ---------------------------------------------------------------------------

func DelHistory(c *gin.Context) {
	userID := middleware.GetUserID(c)

	kidStr := c.PostForm("kid")
	if kidStr == "" {
		response.BadRequest(c, "kid is required")
		return
	}

	// kid is the aid (oid) of the history entry
	kid, _ := strconv.ParseInt(kidStr, 10, 64)
	if kid == 0 {
		// kid might be in "archive_123456" format — try to parse after underscore
		// but for self-hosted, we just use the numeric aid
		response.BadRequest(c, "invalid kid")
		return
	}

	database.DB.Where("user_id = ? AND aid = ?", userID, kid).Delete(&model.WatchHistory{})
	response.Success(c, nil)
}

// ---------------------------------------------------------------------------
// POST /x/v2/history/clear  — 清空历史记录
// ---------------------------------------------------------------------------

func ClearHistory(c *gin.Context) {
	userID := middleware.GetUserID(c)
	database.DB.Where("user_id = ?", userID).Delete(&model.WatchHistory{})
	response.Success(c, nil)
}

// ---------------------------------------------------------------------------
// POST /x/v2/history/shadow/set  — 暂停/恢复历史记录
// ---------------------------------------------------------------------------

func HistoryShadowSet(c *gin.Context) {
	userID := middleware.GetUserID(c)

	switchStr := c.PostForm("switch")
	paused := 0
	if switchStr == "true" || switchStr == "1" {
		paused = 1
	}

	var settings model.UserSettings
	result := database.DB.Where("user_id = ?", userID).First(&settings)
	if result.Error == gorm.ErrRecordNotFound {
		settings = model.UserSettings{
			UserID:        userID,
			HistoryPaused: paused,
		}
		database.DB.Create(&settings)
	} else {
		database.DB.Model(&settings).Update("history_paused", paused)
	}

	response.Success(c, nil)
}

// ---------------------------------------------------------------------------
// GET /x/v2/history/shadow  — 查询历史记录暂停状态
// ---------------------------------------------------------------------------

func HistoryShadow(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var settings model.UserSettings
	result := database.DB.Where("user_id = ?", userID).First(&settings)

	paused := false
	if result.Error == nil && settings.HistoryPaused == 1 {
		paused = true
	}

	response.Success(c, paused)
}

// ---------------------------------------------------------------------------
// POST /x/click-interface/web/heartbeat  — 上报播放进度
// ---------------------------------------------------------------------------

func HeartBeat(c *gin.Context) {
	userID := middleware.GetUserID(c)

	// Check if history is paused
	var settings model.UserSettings
	if err := database.DB.Where("user_id = ?", userID).First(&settings).Error; err == nil {
		if settings.HistoryPaused == 1 {
			response.Success(c, nil)
			return
		}
	}

	bvid := c.PostForm("bvid")
	aidStr := c.PostForm("aid")
	cidStr := c.PostForm("cid")
	progressStr := c.PostForm("played_time")
	epidStr := c.PostForm("epid")
	sidStr := c.PostForm("sid")
	typeStr := c.PostForm("type")
	subTypeStr := c.PostForm("sub_type")

	var aid int64
	if aidStr != "" {
		aid, _ = strconv.ParseInt(aidStr, 10, 64)
	}
	cid, _ := strconv.ParseInt(cidStr, 10, 64)
	progress, _ := strconv.Atoi(progressStr)
	epid, _ := strconv.ParseInt(epidStr, 10, 64)
	sid, _ := strconv.ParseInt(sidStr, 10, 64)

	// Resolve bvid → aid early so the DB lookup uses the real aid
	if aid == 0 && bvid != "" {
		if info, err := bilibili.FetchVideoInfo(0, bvid); err == nil && info != nil && info.Aid != 0 {
			aid = info.Aid
		}
	}

	// Determine business type from type parameter
	business := "archive"
	if typeStr == "4" || epid > 0 {
		business = "pgc"
	}
	_ = subTypeStr // reserved for future use

	now := time.Now().Unix()

	// Try to find existing record
	var existing model.WatchHistory
	result := database.DB.Where("user_id = ? AND aid = ?", userID, aid).First(&existing)

	if result.Error == gorm.ErrRecordNotFound {
		// New entry — fetch video metadata from Bilibili
		info, _ := bilibili.FetchVideoInfo(aid, bvid)

		title := ""
		pic := ""
		duration := 0
		ownerMid := int64(0)
		ownerName := ""
		ownerFace := ""
		videos := 0
		fetchedBvid := bvid

		if info != nil {
			title = info.Title
			pic = info.Pic
			duration = info.Duration
			ownerMid = info.OwnerMid
			ownerName = info.OwnerName
			ownerFace = info.OwnerFace
			videos = info.Videos
			if info.Bvid != "" {
				fetchedBvid = info.Bvid
			}
			if info.Aid != 0 {
				aid = info.Aid
			}
		}

		entry := model.WatchHistory{
			UserID:     userID,
			Aid:        aid,
			Bvid:       fetchedBvid,
			Cid:        cid,
			Epid:       epid,
			SeasonID:   sid,
			Title:      title,
			Cover:      pic,
			Duration:   duration,
			Progress:   progress,
			AuthorMid:  ownerMid,
			AuthorName: ownerName,
			AuthorFace: ownerFace,
			Badge:      badgeFromBusiness(business),
			Kid:        aidStr,
			Business:   business,
			ViewAt:     now,
			Videos:     videos,
		}
		database.DB.Create(&entry)
	} else {
		// Update existing record
		updates := map[string]interface{}{
			"progress": progress,
			"view_at":  now,
		}
		if cid > 0 {
			updates["cid"] = cid
		}
		if epid > 0 {
			updates["epid"] = epid
		}
		if sid > 0 {
			updates["season_id"] = sid
		}
		database.DB.Model(&existing).Updates(updates)
	}

	response.Success(c, nil)
}

// ---------------------------------------------------------------------------
// POST /x/v2/history/report  — 历史上报（添加历史记录）
// ---------------------------------------------------------------------------

func HistoryReport(c *gin.Context) {
	userID := middleware.GetUserID(c)

	// Check if history is paused
	var settings model.UserSettings
	if err := database.DB.Where("user_id = ?", userID).First(&settings).Error; err == nil {
		if settings.HistoryPaused == 1 {
			response.Success(c, nil)
			return
		}
	}

	aidStr := c.PostForm("aid")
	typeStr := c.PostForm("type")

	aid, _ := strconv.ParseInt(aidStr, 10, 64)
	if aid == 0 {
		response.BadRequest(c, "aid is required")
		return
	}

	business := "archive"
	if typeStr == "4" {
		business = "pgc"
	}

	now := time.Now().Unix()

	// Check if already exists
	var existing model.WatchHistory
	result := database.DB.Where("user_id = ? AND aid = ?", userID, aid).First(&existing)

	if result.Error == gorm.ErrRecordNotFound {
		// Fetch video info
		info, _ := bilibili.FetchVideoInfo(aid, "")

		title := ""
		pic := ""
		duration := 0
		ownerMid := int64(0)
		ownerName := ""
		ownerFace := ""
		videos := 0
		bvid := ""

		if info != nil {
			title = info.Title
			pic = info.Pic
			duration = info.Duration
			ownerMid = info.OwnerMid
			ownerName = info.OwnerName
			ownerFace = info.OwnerFace
			videos = info.Videos
			bvid = info.Bvid
		}

		entry := model.WatchHistory{
			UserID:     userID,
			Aid:        aid,
			Bvid:       bvid,
			Title:      title,
			Cover:      pic,
			Duration:   duration,
			AuthorMid:  ownerMid,
			AuthorName: ownerName,
			AuthorFace: ownerFace,
			Badge:      badgeFromBusiness(business),
			Kid:        aidStr,
			Business:   business,
			ViewAt:     now,
			Videos:     videos,
		}
		database.DB.Create(&entry)
	} else {
		database.DB.Model(&existing).Updates(map[string]interface{}{
			"view_at": now,
		})
	}

	response.Success(c, nil)
}

func badgeFromBusiness(business string) string {
	switch business {
	case "pgc":
		return "番剧"
	case "live":
		return "直播"
	case "article":
		return "专栏"
	default:
		return ""
	}
}

// ---------------------------------------------------------------------------
// POST /x/v1/medialist/history  — 媒体列表历史上报
// ---------------------------------------------------------------------------

func MedialistHistory(c *gin.Context) {
	userID := middleware.GetUserID(c)

	oidStr := c.PostForm("oid")
	oid, _ := strconv.ParseInt(oidStr, 10, 64)
	if oid == 0 {
		response.BadRequest(c, "oid is required")
		return
	}

	now := time.Now().Unix()

	// Simply update view_at for the given aid (oid)
	result := database.DB.Model(&model.WatchHistory{}).
		Where("user_id = ? AND aid = ?", userID, oid).
		Update("view_at", now)

	if result.RowsAffected == 0 {
		// Entry doesn't exist yet — create a minimal one
		info, _ := bilibili.FetchVideoInfo(oid, "")
		entry := model.WatchHistory{
			UserID:   userID,
			Aid:      oid,
			Business: "archive",
			ViewAt:   now,
		}
		if info != nil {
			entry.Bvid = info.Bvid
			entry.Title = info.Title
			entry.Cover = info.Pic
			entry.Duration = info.Duration
			entry.AuthorMid = info.OwnerMid
			entry.AuthorName = info.OwnerName
			entry.AuthorFace = info.OwnerFace
			entry.Videos = info.Videos
		}
		database.DB.Create(&entry)
	}

	response.Success(c, nil)
}

// ---------------------------------------------------------------------------
// GET /x/v2/history/progress  — 查询指定视频的观看进度
// ---------------------------------------------------------------------------

func HistoryProgress(c *gin.Context) {
	userID := middleware.GetUserID(c)

	aidStr := c.DefaultQuery("aid", "0")
	bvid := c.DefaultQuery("bvid", "")

	aid, _ := strconv.ParseInt(aidStr, 10, 64)

	// Resolve bvid → aid if needed
	if aid == 0 && bvid != "" {
		if info, err := bilibili.FetchVideoInfo(0, bvid); err == nil && info != nil && info.Aid != 0 {
			aid = info.Aid
		}
	}

	if aid == 0 {
		response.Success(c, gin.H{
			"last_play_time": -1,
			"last_play_cid":  0,
		})
		return
	}

	var entry model.WatchHistory
	result := database.DB.Where("user_id = ? AND aid = ?", userID, aid).First(&entry)
	if result.Error != nil {
		response.Success(c, gin.H{
			"last_play_time": -1,
			"last_play_cid":  0,
		})
		return
	}

	response.Success(c, gin.H{
		"last_play_time": entry.Progress * 1000, // seconds → milliseconds
		"last_play_cid":  entry.Cid,
	})
}
