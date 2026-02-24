package handler

import (
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"piliminusb/bilibili"
	"piliminusb/database"
	"piliminusb/middleware"
	"piliminusb/model"
	"piliminusb/response"
)

// GET /x/v2/history/toview/web
func ToviewList(c *gin.Context) {
	userID := middleware.GetUserID(c)

	pn, _ := strconv.Atoi(c.DefaultQuery("pn", "1"))
	ps, _ := strconv.Atoi(c.DefaultQuery("ps", "20"))
	viewed, _ := strconv.Atoi(c.DefaultQuery("viewed", "0"))
	keyword := c.DefaultQuery("key", "")
	ascStr := c.DefaultQuery("asc", "false")
	asc := ascStr == "true" || ascStr == "1"

	offset := (pn - 1) * ps

	query := database.DB.Where("user_id = ?", userID)

	// viewed: 0=all, 2=unwatched (progress >= 0 and not finished)
	if viewed == 2 {
		query = query.Where("viewed = 0")
	}

	// keyword search
	if keyword != "" {
		query = query.Where("title LIKE ?", "%"+keyword+"%")
	}

	// count
	var count int64
	query.Model(&model.WatchLater{}).Count(&count)

	// order
	order := "added_at DESC"
	if asc {
		order = "added_at ASC"
	}

	var items []model.WatchLater
	query.Order(order).Offset(offset).Limit(ps).Find(&items)

	list := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		list = append(list, item.ToBiliJSON())
	}

	response.Success(c, gin.H{
		"count": count,
		"list":  list,
	})
}

// POST /x/v2/history/toview/add
func ToviewAdd(c *gin.Context) {
	userID := middleware.GetUserID(c)

	aidStr := c.PostForm("aid")
	bvid := c.PostForm("bvid")

	var aid int64
	if aidStr != "" {
		aid, _ = strconv.ParseInt(aidStr, 10, 64)
	}

	if aid == 0 && bvid == "" {
		response.BadRequest(c, "aid or bvid is required")
		return
	}

	// Fetch video metadata from Bilibili
	info, err := bilibili.FetchVideoInfo(aid, bvid)
	if err != nil {
		response.InternalError(c, "failed to fetch video info: "+err.Error())
		return
	}

	now := time.Now().Unix()

	item := model.WatchLater{
		UserID:    userID,
		Aid:       info.Aid,
		Bvid:      info.Bvid,
		Title:     info.Title,
		Pic:       info.Pic,
		Duration:  info.Duration,
		OwnerMid:  info.OwnerMid,
		OwnerName: info.OwnerName,
		OwnerFace: info.OwnerFace,
		Videos:    info.Videos,
		Cid:       info.Cid,
		Pubdate:   info.Pubdate,
		AddedAt:   now,
	}

	// Upsert: if exists, update added_at
	result := database.DB.Where("user_id = ? AND aid = ?", userID, info.Aid).First(&model.WatchLater{})
	if result.Error == gorm.ErrRecordNotFound {
		database.DB.Create(&item)
	} else {
		database.DB.Model(&model.WatchLater{}).
			Where("user_id = ? AND aid = ?", userID, info.Aid).
			Updates(map[string]interface{}{
				"added_at":   now,
				"title":      info.Title,
				"pic":        info.Pic,
				"owner_name": info.OwnerName,
				"owner_face": info.OwnerFace,
			})
	}

	response.Success(c, nil)
}

// POST /x/v2/history/toview/v2/dels
func ToviewDel(c *gin.Context) {
	userID := middleware.GetUserID(c)

	resources := c.PostForm("resources")
	if resources == "" {
		response.BadRequest(c, "resources is required")
		return
	}

	aidStrs := strings.Split(resources, ",")
	aids := make([]int64, 0, len(aidStrs))
	for _, s := range aidStrs {
		s = strings.TrimSpace(s)
		if aid, err := strconv.ParseInt(s, 10, 64); err == nil {
			aids = append(aids, aid)
		}
	}

	if len(aids) == 0 {
		response.BadRequest(c, "no valid aids provided")
		return
	}

	database.DB.Where("user_id = ? AND aid IN ?", userID, aids).Delete(&model.WatchLater{})

	response.Success(c, nil)
}

// GET /x/v2/medialist/resource/list (type=2: watch later, type=3: favorites)
func MediaList(c *gin.Context) {
	typeStr := c.DefaultQuery("type", "2")
	mediaType, _ := strconv.Atoi(typeStr)

	switch mediaType {
	case 3:
		mediaListFav(c)
	default:
		mediaListWatchLater(c)
	}
}

// mediaListFav handles type=3 (favorites folder content).
func mediaListFav(c *gin.Context) {
	userID := middleware.GetUserID(c)

	bizIDStr := c.DefaultQuery("biz_id", "0")
	bizID, _ := strconv.ParseInt(bizIDStr, 10, 64)
	ps, _ := strconv.Atoi(c.DefaultQuery("ps", "20"))
	oidStr := c.DefaultQuery("oid", "")
	descStr := c.DefaultQuery("desc", "true")
	directionStr := c.DefaultQuery("direction", "false")
	withCurrentStr := c.DefaultQuery("with_current", "false")

	desc := descStr == "true" || descStr == "1"
	direction := directionStr == "true" || directionStr == "1"
	withCurrent := withCurrentStr == "true" || withCurrentStr == "1"

	query := database.DB.Where("user_id = ? AND media_id = ?", userID, bizID)

	if oidStr != "" && !withCurrent {
		oid, err := strconv.ParseInt(oidStr, 10, 64)
		if err == nil && oid > 0 {
			var cursor model.FavResource
			if err := database.DB.Where("user_id = ? AND media_id = ? AND resource_id = ?", userID, bizID, oid).
				First(&cursor).Error; err == nil {
				if direction {
					if desc {
						query = query.Where("fav_time > ?", cursor.FavTime)
					} else {
						query = query.Where("fav_time < ?", cursor.FavTime)
					}
				} else {
					if desc {
						query = query.Where("fav_time < ?", cursor.FavTime)
					} else {
						query = query.Where("fav_time > ?", cursor.FavTime)
					}
				}
			}
		}
	}

	var totalCount int64
	database.DB.Model(&model.FavResource{}).
		Where("user_id = ? AND media_id = ?", userID, bizID).Count(&totalCount)

	order := "fav_time DESC"
	if !desc {
		order = "fav_time ASC"
	}
	if direction {
		if desc {
			order = "fav_time ASC"
		} else {
			order = "fav_time DESC"
		}
	}

	var items []model.FavResource
	query.Order(order).Limit(ps + 1).Find(&items)

	hasMore := len(items) > ps
	if hasMore {
		items = items[:ps]
	}

	if direction {
		for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
			items[i], items[j] = items[j], items[i]
		}
	}

	mediaList := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		mediaList = append(mediaList, item.ToMediaListJSON())
	}

	response.Success(c, gin.H{
		"media_list":  mediaList,
		"has_more":    hasMore,
		"total_count": totalCount,
	})
}

// mediaListWatchLater handles type=2 (watch later).
func mediaListWatchLater(c *gin.Context) {
	userID := middleware.GetUserID(c)

	ps, _ := strconv.Atoi(c.DefaultQuery("ps", "20"))
	oidStr := c.DefaultQuery("oid", "")
	descStr := c.DefaultQuery("desc", "true")
	directionStr := c.DefaultQuery("direction", "false")
	withCurrentStr := c.DefaultQuery("with_current", "false")

	desc := descStr == "true" || descStr == "1"
	direction := directionStr == "true" || directionStr == "1"
	withCurrent := withCurrentStr == "true" || withCurrentStr == "1"

	query := database.DB.Where("user_id = ?", userID)

	// Cursor-based pagination using oid (aid of boundary item).
	// When with_current=true this is the initial load — return all items
	// from the beginning (oid just marks the current video, not a filter).
	if oidStr != "" && !withCurrent {
		oid, err := strconv.ParseInt(oidStr, 10, 64)
		if err == nil && oid > 0 {
			var cursor model.WatchLater
			if err := database.DB.Where("user_id = ? AND aid = ?", userID, oid).First(&cursor).Error; err == nil {
				if direction {
					// Load previous
					if desc {
						query = query.Where("added_at > ?", cursor.AddedAt)
					} else {
						query = query.Where("added_at < ?", cursor.AddedAt)
					}
				} else {
					// Load next (with_current is always false here)
					if desc {
						query = query.Where("added_at < ?", cursor.AddedAt)
					} else {
						query = query.Where("added_at > ?", cursor.AddedAt)
					}
				}
			}
		}
	}

	// Count total
	var totalCount int64
	database.DB.Where("user_id = ?", userID).Model(&model.WatchLater{}).Count(&totalCount)

	// Order: for normal loads use the requested sort order.
	// For direction=true (load previous), reverse the sort so we fetch items
	// closest to the cursor, then reverse the results back before returning.
	order := "added_at DESC"
	if !desc {
		order = "added_at ASC"
	}
	if direction {
		if desc {
			order = "added_at ASC"
		} else {
			order = "added_at DESC"
		}
	}

	var items []model.WatchLater
	query.Order(order).Limit(ps + 1).Find(&items)

	hasMore := len(items) > ps
	if hasMore {
		items = items[:ps]
	}

	// Reverse results back to original order so client can prepend directly.
	if direction {
		for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
			items[i], items[j] = items[j], items[i]
		}
	}

	mediaList := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		mediaList = append(mediaList, item.ToMediaListJSON())
	}

	response.Success(c, gin.H{
		"media_list":  mediaList,
		"has_more":    hasMore,
		"total_count": totalCount,
	})
}

// POST /x/v2/history/toview/clear
func ToviewClear(c *gin.Context) {
	userID := middleware.GetUserID(c)

	cleanType := c.PostForm("clean_type")

	query := database.DB.Where("user_id = ?", userID)

	switch cleanType {
	case "2":
		// Clear watched only
		query = query.Where("viewed = 1")
	case "1":
		// Clear invalid — no real concept here, do nothing
		response.Success(c, nil)
		return
	default:
		// Clear all
	}

	query.Delete(&model.WatchLater{})

	response.Success(c, nil)
}
