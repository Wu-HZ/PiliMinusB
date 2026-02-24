package handler

import (
	"encoding/json"
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

// ===========================================================================
// Phase 3 – Favorite Folder Management
// ===========================================================================

// nextMediaID returns the next available media_id for the given user.
func nextMediaID(userID uint) int64 {
	var maxID int64
	database.DB.Model(&model.FavFolder{}).Where("user_id = ?", userID).
		Select("COALESCE(MAX(media_id), 0)").Scan(&maxID)
	return maxID + 1
}

// refreshMediaCount recalculates media_count for a folder from the actual row count.
func refreshMediaCount(userID uint, mediaID int64) {
	var cnt int64
	database.DB.Model(&model.FavResource{}).
		Where("user_id = ? AND media_id = ?", userID, mediaID).Count(&cnt)
	database.DB.Model(&model.FavFolder{}).
		Where("user_id = ? AND media_id = ?", userID, mediaID).
		Update("media_count", cnt)
}

// ownerMidFromUser is a placeholder: in a single-user setup the owner mid can
// simply mirror the local user_id. If a Bilibili mid is stored on the User
// model this can be replaced later.
func ownerMidFromUser(userID uint) int64 {
	return int64(userID)
}

// ---------------------------------------------------------------------------
// GET /x/v3/fav/folder/created/list-all
// ---------------------------------------------------------------------------

func AllFavFolders(c *gin.Context) {
	userID := middleware.GetUserID(c)
	ownerMid := ownerMidFromUser(userID)

	// Optional: rid (resource aid) to mark which folders contain this resource.
	ridStr := c.DefaultQuery("rid", "0")
	rid, _ := strconv.ParseInt(ridStr, 10, 64)

	var folders []model.FavFolder
	database.DB.Where("user_id = ?", userID).Order("sort_order ASC, media_id ASC").Find(&folders)

	// If rid is provided, find which folders contain it.
	favSet := map[int64]bool{}
	if rid > 0 {
		var ress []model.FavResource
		database.DB.Where("user_id = ? AND resource_id = ?", userID, rid).Find(&ress)
		for _, r := range ress {
			favSet[r.MediaID] = true
		}
	}

	list := make([]map[string]interface{}, 0, len(folders))
	for _, f := range folders {
		state := 0
		if favSet[f.MediaID] {
			state = 1
		}
		list = append(list, f.ToBiliJSONWithFavState(ownerMid, state))
	}

	response.Success(c, gin.H{
		"count":    len(list),
		"list":     list,
		"has_more": false,
	})
}

// ---------------------------------------------------------------------------
// GET /x/v3/fav/folder/created/list
// ---------------------------------------------------------------------------

func ListFavFolders(c *gin.Context) {
	userID := middleware.GetUserID(c)
	ownerMid := ownerMidFromUser(userID)

	pn, _ := strconv.Atoi(c.DefaultQuery("pn", "1"))
	ps, _ := strconv.Atoi(c.DefaultQuery("ps", "20"))
	offset := (pn - 1) * ps

	var total int64
	database.DB.Model(&model.FavFolder{}).Where("user_id = ?", userID).Count(&total)

	var folders []model.FavFolder
	database.DB.Where("user_id = ?", userID).
		Order("sort_order ASC, media_id ASC").Offset(offset).Limit(ps).Find(&folders)

	list := make([]map[string]interface{}, 0, len(folders))
	for _, f := range folders {
		list = append(list, f.ToBiliJSON(ownerMid))
	}

	response.Success(c, gin.H{
		"count":    total,
		"list":     list,
		"has_more": int64(offset+ps) < total,
	})
}

// ---------------------------------------------------------------------------
// GET /x/v3/fav/folder/info
// ---------------------------------------------------------------------------

func FavFolderInfo(c *gin.Context) {
	userID := middleware.GetUserID(c)
	ownerMid := ownerMidFromUser(userID)

	mediaIDStr := c.Query("media_id")
	mediaID, _ := strconv.ParseInt(mediaIDStr, 10, 64)
	if mediaID == 0 {
		response.BadRequest(c, "media_id is required")
		return
	}

	var folder model.FavFolder
	if err := database.DB.Where("user_id = ? AND media_id = ?", userID, mediaID).First(&folder).Error; err != nil {
		response.Error(c, 404, -404, "folder not found")
		return
	}

	response.Success(c, folder.ToBiliJSON(ownerMid))
}

// ---------------------------------------------------------------------------
// POST /x/v3/fav/folder/add
// ---------------------------------------------------------------------------

func AddFavFolder(c *gin.Context) {
	userID := middleware.GetUserID(c)

	title := c.PostForm("title")
	intro := c.PostForm("intro")
	if title == "" {
		response.BadRequest(c, "title is required")
		return
	}

	now := time.Now().Unix()
	mediaID := nextMediaID(userID)

	// First folder becomes the default.
	isDefault := 0
	var cnt int64
	database.DB.Model(&model.FavFolder{}).Where("user_id = ?", userID).Count(&cnt)
	if cnt == 0 {
		isDefault = 1
	}

	folder := model.FavFolder{
		UserID:    userID,
		MediaID:   mediaID,
		Title:     title,
		Intro:     intro,
		Ctime:     now,
		Mtime:     now,
		SortOrder: int(cnt), // append at end
		IsDefault: isDefault,
	}
	database.DB.Create(&folder)

	ownerMid := ownerMidFromUser(userID)
	response.Success(c, folder.ToBiliJSON(ownerMid))
}

// ---------------------------------------------------------------------------
// POST /x/v3/fav/folder/edit
// ---------------------------------------------------------------------------

func EditFavFolder(c *gin.Context) {
	userID := middleware.GetUserID(c)

	mediaIDStr := c.PostForm("media_id")
	mediaID, _ := strconv.ParseInt(mediaIDStr, 10, 64)
	if mediaID == 0 {
		response.BadRequest(c, "media_id is required")
		return
	}

	var folder model.FavFolder
	if err := database.DB.Where("user_id = ? AND media_id = ?", userID, mediaID).First(&folder).Error; err != nil {
		response.Error(c, 404, -404, "folder not found")
		return
	}

	title := c.PostForm("title")
	intro := c.PostForm("intro")
	cover := c.PostForm("cover")

	updates := map[string]interface{}{"mtime": time.Now().Unix()}
	if title != "" {
		updates["title"] = title
	}
	if intro != "" {
		updates["intro"] = intro
	}
	if cover != "" {
		updates["cover"] = cover
	}

	database.DB.Model(&model.FavFolder{}).
		Where("user_id = ? AND media_id = ?", userID, mediaID).Updates(updates)

	response.Success(c, nil)
}

// ---------------------------------------------------------------------------
// POST /x/v3/fav/folder/del
// ---------------------------------------------------------------------------

func DelFavFolder(c *gin.Context) {
	userID := middleware.GetUserID(c)

	mediaIDsStr := c.PostForm("media_ids")
	if mediaIDsStr == "" {
		response.BadRequest(c, "media_ids is required")
		return
	}

	ids := parseIntList(mediaIDsStr)
	if len(ids) == 0 {
		response.BadRequest(c, "no valid media_ids")
		return
	}

	// Delete folder contents and the folders themselves.
	database.DB.Where("user_id = ? AND media_id IN ?", userID, ids).Delete(&model.FavResource{})
	database.DB.Where("user_id = ? AND media_id IN ?", userID, ids).Delete(&model.FavFolder{})

	response.Success(c, nil)
}

// ---------------------------------------------------------------------------
// POST /x/v3/fav/folder/sort
// ---------------------------------------------------------------------------

func SortFavFolder(c *gin.Context) {
	userID := middleware.GetUserID(c)

	idsStr := c.PostForm("sort")
	if idsStr == "" {
		idsStr = c.PostForm("media_ids")
	}
	if idsStr == "" {
		response.BadRequest(c, "sort is required")
		return
	}

	ids := parseIntList(idsStr)
	for i, id := range ids {
		database.DB.Model(&model.FavFolder{}).
			Where("user_id = ? AND media_id = ?", userID, id).
			Update("sort_order", i)
	}

	response.Success(c, nil)
}

// ===========================================================================
// Phase 3 – Favorite Resource (Content) Management
// ===========================================================================

// ---------------------------------------------------------------------------
// GET /x/v3/fav/resource/list
// ---------------------------------------------------------------------------

func ListFavResources(c *gin.Context) {
	userID := middleware.GetUserID(c)
	ownerMid := ownerMidFromUser(userID)

	mediaIDStr := c.Query("media_id")
	mediaID, _ := strconv.ParseInt(mediaIDStr, 10, 64)
	if mediaID == 0 {
		response.BadRequest(c, "media_id is required")
		return
	}

	pn, _ := strconv.Atoi(c.DefaultQuery("pn", "1"))
	ps, _ := strconv.Atoi(c.DefaultQuery("ps", "20"))
	keyword := c.DefaultQuery("keyword", "")
	order := c.DefaultQuery("order", "mtime")
	offset := (pn - 1) * ps

	// Folder info
	var folder model.FavFolder
	if err := database.DB.Where("user_id = ? AND media_id = ?", userID, mediaID).First(&folder).Error; err != nil {
		response.Error(c, 404, -404, "folder not found")
		return
	}

	query := database.DB.Where("user_id = ? AND media_id = ?", userID, mediaID)

	if keyword != "" {
		query = query.Where("title LIKE ?", "%"+keyword+"%")
	}

	// Order
	orderClause := "fav_time DESC"
	switch order {
	case "mtime":
		orderClause = "fav_time DESC"
	case "view":
		orderClause = "fav_time DESC" // no view count, fallback
	case "pubtime":
		orderClause = "pubtime DESC"
	}

	var items []model.FavResource
	query.Order(orderClause).Offset(offset).Limit(ps).Find(&items)

	var total int64
	database.DB.Model(&model.FavResource{}).
		Where("user_id = ? AND media_id = ?", userID, mediaID).Count(&total)

	medias := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		medias = append(medias, item.ToBiliJSON())
	}

	response.Success(c, gin.H{
		"info":     folder.ToBiliJSON(ownerMid),
		"medias":   medias,
		"has_more": int64(offset+ps) < total,
		"ttl":      1,
	})
}

// ---------------------------------------------------------------------------
// POST /x/v3/fav/resource/batch-deal
// ---------------------------------------------------------------------------

func BatchDealFav(c *gin.Context) {
	userID := middleware.GetUserID(c)

	resourcesStr := c.PostForm("resources")
	addIdsStr := c.PostForm("add_media_ids")
	delIdsStr := c.PostForm("del_media_ids")

	// Parse resources: can be a JSON array or comma-separated "aid:type" pairs.
	type resItem struct {
		Rid  int64 `json:"rid"`
		Type int   `json:"type"`
	}
	var resources []resItem

	// Try JSON first
	if err := json.Unmarshal([]byte(resourcesStr), &resources); err != nil {
		// Fallback: comma-separated "aid:type"
		for _, part := range strings.Split(resourcesStr, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			segs := strings.SplitN(part, ":", 2)
			rid, _ := strconv.ParseInt(segs[0], 10, 64)
			rtype := 2
			if len(segs) > 1 {
				rtype, _ = strconv.Atoi(segs[1])
			}
			if rid > 0 {
				resources = append(resources, resItem{Rid: rid, Type: rtype})
			}
		}
	}

	if len(resources) == 0 {
		response.BadRequest(c, "resources is required")
		return
	}

	addIds := parseIntList(addIdsStr)
	delIds := parseIntList(delIdsStr)

	now := time.Now().Unix()

	tx := database.DB.Begin()

	// Add resources to target folders
	for _, mid := range addIds {
		for _, res := range resources {
			// Fetch metadata
			info, _ := bilibili.FetchVideoInfo(res.Rid, "")

			fr := model.FavResource{
				UserID:       userID,
				MediaID:      mid,
				ResourceID:   res.Rid,
				ResourceType: res.Type,
				FavTime:      now,
			}
			if info != nil {
				fr.Title = info.Title
				fr.Cover = info.Pic
				fr.Duration = info.Duration
				fr.UpperMid = info.OwnerMid
				fr.UpperName = info.OwnerName
				fr.Bvid = info.Bvid
				fr.Pubtime = info.Pubdate
				fr.Cid = info.Cid
			}

			// Upsert
			var existing model.FavResource
			if tx.Where("user_id = ? AND media_id = ? AND resource_id = ?", userID, mid, res.Rid).
				First(&existing).Error == gorm.ErrRecordNotFound {
				tx.Create(&fr)
			}
		}
	}

	// Remove resources from target folders
	for _, mid := range delIds {
		rids := make([]int64, 0, len(resources))
		for _, res := range resources {
			rids = append(rids, res.Rid)
		}
		tx.Where("user_id = ? AND media_id = ? AND resource_id IN ?", userID, mid, rids).
			Delete(&model.FavResource{})
	}

	if err := tx.Commit().Error; err != nil {
		response.InternalError(c, "batch-deal failed: "+err.Error())
		return
	}

	// Refresh media_count for all affected folders
	for _, mid := range addIds {
		refreshMediaCount(userID, mid)
	}
	for _, mid := range delIds {
		refreshMediaCount(userID, mid)
	}

	response.Success(c, nil)
}

// ---------------------------------------------------------------------------
// POST /x/v3/fav/resource/unfav-all
// ---------------------------------------------------------------------------

func UnfavAll(c *gin.Context) {
	userID := middleware.GetUserID(c)

	ridStr := c.PostForm("rid")
	rid, _ := strconv.ParseInt(ridStr, 10, 64)
	if rid == 0 {
		response.BadRequest(c, "rid is required")
		return
	}

	// Find all folders that contain this resource, then delete.
	var affected []model.FavResource
	database.DB.Where("user_id = ? AND resource_id = ?", userID, rid).Find(&affected)

	affectedMediaIDs := map[int64]bool{}
	for _, r := range affected {
		affectedMediaIDs[r.MediaID] = true
	}

	database.DB.Where("user_id = ? AND resource_id = ?", userID, rid).Delete(&model.FavResource{})

	for mid := range affectedMediaIDs {
		refreshMediaCount(userID, mid)
	}

	response.Success(c, nil)
}

// ---------------------------------------------------------------------------
// POST /x/v3/fav/resource/copy
// ---------------------------------------------------------------------------

func CopyFavResource(c *gin.Context) {
	userID := middleware.GetUserID(c)

	srcMediaIDStr := c.PostForm("src_media_id")
	tarMediaIDStr := c.PostForm("tar_media_id")
	resourcesStr := c.PostForm("resources")

	srcMediaID, _ := strconv.ParseInt(srcMediaIDStr, 10, 64)
	tarMediaID, _ := strconv.ParseInt(tarMediaIDStr, 10, 64)

	if srcMediaID == 0 || tarMediaID == 0 || resourcesStr == "" {
		response.BadRequest(c, "src_media_id, tar_media_id, and resources are required")
		return
	}

	rids := parseIntList(resourcesStr)
	now := time.Now().Unix()

	for _, rid := range rids {
		var src model.FavResource
		if database.DB.Where("user_id = ? AND media_id = ? AND resource_id = ?", userID, srcMediaID, rid).
			First(&src).Error == nil {
			// Copy to target
			var existing model.FavResource
			if database.DB.Where("user_id = ? AND media_id = ? AND resource_id = ?", userID, tarMediaID, rid).
				First(&existing).Error == gorm.ErrRecordNotFound {
				newRes := src
				newRes.ID = 0
				newRes.MediaID = tarMediaID
				newRes.FavTime = now
				database.DB.Create(&newRes)
			}
		}
	}

	refreshMediaCount(userID, tarMediaID)
	response.Success(c, nil)
}

// ---------------------------------------------------------------------------
// POST /x/v3/fav/resource/move
// ---------------------------------------------------------------------------

func MoveFavResource(c *gin.Context) {
	userID := middleware.GetUserID(c)

	srcMediaIDStr := c.PostForm("src_media_id")
	tarMediaIDStr := c.PostForm("tar_media_id")
	resourcesStr := c.PostForm("resources")

	srcMediaID, _ := strconv.ParseInt(srcMediaIDStr, 10, 64)
	tarMediaID, _ := strconv.ParseInt(tarMediaIDStr, 10, 64)

	if srcMediaID == 0 || tarMediaID == 0 || resourcesStr == "" {
		response.BadRequest(c, "src_media_id, tar_media_id, and resources are required")
		return
	}

	rids := parseIntList(resourcesStr)
	now := time.Now().Unix()

	for _, rid := range rids {
		var src model.FavResource
		if database.DB.Where("user_id = ? AND media_id = ? AND resource_id = ?", userID, srcMediaID, rid).
			First(&src).Error == nil {
			// Copy to target
			var existing model.FavResource
			if database.DB.Where("user_id = ? AND media_id = ? AND resource_id = ?", userID, tarMediaID, rid).
				First(&existing).Error == gorm.ErrRecordNotFound {
				newRes := src
				newRes.ID = 0
				newRes.MediaID = tarMediaID
				newRes.FavTime = now
				database.DB.Create(&newRes)
			}
			// Remove from source
			database.DB.Where("user_id = ? AND media_id = ? AND resource_id = ?", userID, srcMediaID, rid).
				Delete(&model.FavResource{})
		}
	}

	refreshMediaCount(userID, srcMediaID)
	refreshMediaCount(userID, tarMediaID)
	response.Success(c, nil)
}

// ---------------------------------------------------------------------------
// POST /x/v3/fav/resource/clean
// ---------------------------------------------------------------------------

func CleanFavResource(c *gin.Context) {
	// Self-hosted server has no concept of "invalid" resources.
	response.Success(c, nil)
}

// ---------------------------------------------------------------------------
// POST /x/v3/fav/resource/sort
// ---------------------------------------------------------------------------

func SortFavResource(c *gin.Context) {
	userID := middleware.GetUserID(c)

	mediaIDStr := c.PostForm("media_id")
	mediaID, _ := strconv.ParseInt(mediaIDStr, 10, 64)
	resourcesStr := c.PostForm("sort")
	if resourcesStr == "" {
		resourcesStr = c.PostForm("resources")
	}

	if mediaID == 0 || resourcesStr == "" {
		response.BadRequest(c, "media_id and resources are required")
		return
	}

	rids := parseIntList(resourcesStr)
	for i, rid := range rids {
		database.DB.Model(&model.FavResource{}).
			Where("user_id = ? AND media_id = ? AND resource_id = ?", userID, mediaID, rid).
			Update("sort_order", i)
	}

	response.Success(c, nil)
}

// ===========================================================================
// Phase 3 – Watch Later ↔ Favorites Cross-Operations
// ===========================================================================

// ---------------------------------------------------------------------------
// POST /x/v2/history/toview/copy  — copy from watch later to a fav folder
// ---------------------------------------------------------------------------

func ToviewCopy(c *gin.Context) {
	userID := middleware.GetUserID(c)

	mediaIDStr := c.PostForm("tar_media_id")
	if mediaIDStr == "" {
		mediaIDStr = c.PostForm("media_id")
	}
	aidsStr := c.PostForm("resources")

	mediaID, _ := strconv.ParseInt(mediaIDStr, 10, 64)
	if mediaID == 0 || aidsStr == "" {
		response.BadRequest(c, "media_id and resources are required")
		return
	}

	aids := parseIntList(aidsStr)
	now := time.Now().Unix()

	for _, aid := range aids {
		var wl model.WatchLater
		if database.DB.Where("user_id = ? AND aid = ?", userID, aid).First(&wl).Error == nil {
			var existing model.FavResource
			if database.DB.Where("user_id = ? AND media_id = ? AND resource_id = ?", userID, mediaID, aid).
				First(&existing).Error == gorm.ErrRecordNotFound {
				fr := model.FavResource{
					UserID:       userID,
					MediaID:      mediaID,
					ResourceID:   wl.Aid,
					ResourceType: 2,
					Title:        wl.Title,
					Cover:        wl.Pic,
					Duration:     wl.Duration,
					UpperMid:     wl.OwnerMid,
					UpperName:    wl.OwnerName,
					Bvid:         wl.Bvid,
					Pubtime:      wl.Pubdate,
					Cid:          wl.Cid,
					FavTime:      now,
				}
				database.DB.Create(&fr)
			}
		}
	}

	refreshMediaCount(userID, mediaID)
	response.Success(c, nil)
}

// ---------------------------------------------------------------------------
// POST /x/v2/history/toview/move  — move from watch later to a fav folder
// ---------------------------------------------------------------------------

func ToviewMove(c *gin.Context) {
	userID := middleware.GetUserID(c)

	mediaIDStr := c.PostForm("tar_media_id")
	if mediaIDStr == "" {
		mediaIDStr = c.PostForm("media_id")
	}
	aidsStr := c.PostForm("resources")

	mediaID, _ := strconv.ParseInt(mediaIDStr, 10, 64)
	if mediaID == 0 || aidsStr == "" {
		response.BadRequest(c, "media_id and resources are required")
		return
	}

	aids := parseIntList(aidsStr)
	now := time.Now().Unix()

	for _, aid := range aids {
		var wl model.WatchLater
		if database.DB.Where("user_id = ? AND aid = ?", userID, aid).First(&wl).Error == nil {
			var existing model.FavResource
			if database.DB.Where("user_id = ? AND media_id = ? AND resource_id = ?", userID, mediaID, aid).
				First(&existing).Error == gorm.ErrRecordNotFound {
				fr := model.FavResource{
					UserID:       userID,
					MediaID:      mediaID,
					ResourceID:   wl.Aid,
					ResourceType: 2,
					Title:        wl.Title,
					Cover:        wl.Pic,
					Duration:     wl.Duration,
					UpperMid:     wl.OwnerMid,
					UpperName:    wl.OwnerName,
					Bvid:         wl.Bvid,
					Pubtime:      wl.Pubdate,
					Cid:          wl.Cid,
					FavTime:      now,
				}
				database.DB.Create(&fr)
			}
			// Remove from watch later
			database.DB.Where("user_id = ? AND aid = ?", userID, aid).Delete(&model.WatchLater{})
		}
	}

	refreshMediaCount(userID, mediaID)
	response.Success(c, nil)
}

// ===========================================================================
// Helpers
// ===========================================================================

// parseIntList splits a comma-separated string into int64 values.
func parseIntList(s string) []int64 {
	parts := strings.Split(s, ",")
	result := make([]int64, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if v, err := strconv.ParseInt(p, 10, 64); err == nil && v > 0 {
			result = append(result, v)
		}
	}
	return result
}
