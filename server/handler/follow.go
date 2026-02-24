package handler

import (
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"piliminusb/database"
	"piliminusb/middleware"
	"piliminusb/model"
	"piliminusb/response"
)

// ===========================================================================
// Phase 4 – Following Management
// ===========================================================================

// nextTagID returns the next available tag_id for the given user.
func nextTagID(userID uint) int64 {
	var maxID int64
	database.DB.Model(&model.FollowTag{}).Where("user_id = ?", userID).
		Select("COALESCE(MAX(tag_id), 0)").Scan(&maxID)
	return maxID + 1
}

// refreshTagCount recalculates count for a tag.
func refreshTagCount(userID uint, tagID int64) {
	var cnt int64
	database.DB.Model(&model.FollowTagMember{}).
		Where("user_id = ? AND tag_id = ?", userID, tagID).Count(&cnt)
	database.DB.Model(&model.FollowTag{}).
		Where("user_id = ? AND tag_id = ?", userID, tagID).
		Update("count", cnt)
}

// ---------------------------------------------------------------------------
// GET /x/relation  — query relation status for a given fid
// ---------------------------------------------------------------------------

func Relation(c *gin.Context) {
	userID := middleware.GetUserID(c)

	fidStr := c.Query("fid")
	fid, _ := strconv.ParseInt(fidStr, 10, 64)
	if fid == 0 {
		response.BadRequest(c, "fid is required")
		return
	}

	var f model.Following
	attribute := 0
	special := 0
	if database.DB.Where("user_id = ? AND mid = ?", userID, fid).First(&f).Error == nil {
		attribute = int(f.Attribute)
		if f.IsSpecial == 1 {
			special = 1
		}
	}

	// Collect tag IDs that contain this follow
	var members []model.FollowTagMember
	database.DB.Where("user_id = ? AND follow_mid = ?", userID, fid).Find(&members)
	tagIDs := make([]int64, 0, len(members))
	for _, m := range members {
		tagIDs = append(tagIDs, m.TagID)
	}

	response.Success(c, gin.H{
		"attribute": attribute,
		"special":   special,
		"tag":       tagIDs,
	})
}

// ---------------------------------------------------------------------------
// GET /x/relation/followings
// ---------------------------------------------------------------------------

func Followings(c *gin.Context) {
	userID := middleware.GetUserID(c)

	pn, _ := strconv.Atoi(c.DefaultQuery("pn", "1"))
	ps, _ := strconv.Atoi(c.DefaultQuery("ps", "20"))
	orderType := c.DefaultQuery("order_type", "")
	offset := (pn - 1) * ps

	var total int64
	database.DB.Model(&model.Following{}).Where("user_id = ?", userID).Count(&total)

	orderClause := "m_time DESC"
	if orderType == "attention" {
		orderClause = "is_special DESC, m_time DESC"
	}

	var follows []model.Following
	database.DB.Where("user_id = ?", userID).
		Order(orderClause).Offset(offset).Limit(ps).Find(&follows)

	list := make([]map[string]interface{}, 0, len(follows))
	for _, f := range follows {
		list = append(list, f.ToBiliJSON())
	}

	response.Success(c, gin.H{
		"list":  list,
		"re_version": 0,
		"total": total,
	})
}

// ---------------------------------------------------------------------------
// GET /x/relation/followings/search
// ---------------------------------------------------------------------------

func FollowingsSearch(c *gin.Context) {
	userID := middleware.GetUserID(c)

	pn, _ := strconv.Atoi(c.DefaultQuery("pn", "1"))
	ps, _ := strconv.Atoi(c.DefaultQuery("ps", "20"))
	name := c.DefaultQuery("name", "")
	offset := (pn - 1) * ps

	query := database.DB.Model(&model.Following{}).Where("user_id = ?", userID)
	if name != "" {
		query = query.Where("name LIKE ?", "%"+name+"%")
	}

	var total int64
	query.Count(&total)

	var follows []model.Following
	query.Order("m_time DESC").Offset(offset).Limit(ps).Find(&follows)

	list := make([]map[string]interface{}, 0, len(follows))
	for _, f := range follows {
		list = append(list, f.ToBiliJSON())
	}

	response.Success(c, gin.H{
		"list":  list,
		"total": total,
	})
}

// ---------------------------------------------------------------------------
// POST /x/relation/modify  — follow / unfollow / block / unblock
// ---------------------------------------------------------------------------

func RelationMod(c *gin.Context) {
	userID := middleware.GetUserID(c)

	fidStr := c.PostForm("fid")
	actStr := c.PostForm("act")
	fid, _ := strconv.ParseInt(fidStr, 10, 64)
	act, _ := strconv.Atoi(actStr)

	if fid == 0 {
		response.BadRequest(c, "fid is required")
		return
	}

	now := time.Now().Unix()

	switch act {
	case 1: // follow
		uname := c.PostForm("uname")
		face := c.PostForm("face")
		var existing model.Following
		if database.DB.Where("user_id = ? AND mid = ?", userID, fid).
			First(&existing).Error == gorm.ErrRecordNotFound {
			f := model.Following{
				UserID:    userID,
				Mid:       fid,
				Name:      uname,
				Face:      face,
				Attribute: 2,
				MTime:     now,
			}
			database.DB.Create(&f)
		} else {
			// Update name/face if provided
			updates := map[string]interface{}{}
			if uname != "" {
				updates["name"] = uname
			}
			if face != "" {
				updates["face"] = face
			}
			if len(updates) > 0 {
				database.DB.Model(&model.Following{}).
					Where("user_id = ? AND mid = ?", userID, fid).
					Updates(updates)
			}
		}
	case 2: // unfollow
		// Remove from all tags
		database.DB.Where("user_id = ? AND follow_mid = ?", userID, fid).
			Delete(&model.FollowTagMember{})
		database.DB.Where("user_id = ? AND mid = ?", userID, fid).
			Delete(&model.Following{})
		// Refresh tag counts
		var tags []model.FollowTag
		database.DB.Where("user_id = ?", userID).Find(&tags)
		for _, t := range tags {
			refreshTagCount(userID, t.TagID)
		}
	case 5, 6: // block / unblock — we just do follow/unfollow locally
		if act == 5 {
			database.DB.Where("user_id = ? AND mid = ?", userID, fid).
				Delete(&model.Following{})
		}
	default:
		response.BadRequest(c, "invalid act")
		return
	}

	response.Success(c, nil)
}

// ---------------------------------------------------------------------------
// GET /x/relation/tags
// ---------------------------------------------------------------------------

func FollowTags(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var tags []model.FollowTag
	database.DB.Where("user_id = ?", userID).Order("tag_id ASC").Find(&tags)

	list := make([]map[string]interface{}, 0, len(tags))
	// Prepend the built-in "special follow" virtual tag (tagid = -10)
	var specialCnt int64
	database.DB.Model(&model.Following{}).Where("user_id = ? AND is_special = 1", userID).Count(&specialCnt)
	list = append(list, map[string]interface{}{
		"tagid": -10,
		"name":  "特别关注",
		"count": specialCnt,
		"tip":   "",
	})

	for _, t := range tags {
		list = append(list, t.ToBiliJSON())
	}

	response.Success(c, list)
}

// ---------------------------------------------------------------------------
// GET /x/relation/tag  — members of a tag
// ---------------------------------------------------------------------------

func FollowTagMembers(c *gin.Context) {
	userID := middleware.GetUserID(c)

	tagIDStr := c.Query("tagid")
	tagID, _ := strconv.ParseInt(tagIDStr, 10, 64)
	pn, _ := strconv.Atoi(c.DefaultQuery("pn", "1"))
	ps, _ := strconv.Atoi(c.DefaultQuery("ps", "20"))
	offset := (pn - 1) * ps

	if tagID == -10 {
		// Special follow virtual tag
		var follows []model.Following
		database.DB.Where("user_id = ? AND is_special = 1", userID).
			Order("m_time DESC").Offset(offset).Limit(ps).Find(&follows)

		list := make([]map[string]interface{}, 0, len(follows))
		for _, f := range follows {
			list = append(list, f.ToBiliJSON())
		}

		// Client expects data to be a plain array
		response.Success(c, list)
		return
	}

	// Normal tag
	var members []model.FollowTagMember
	database.DB.Where("user_id = ? AND tag_id = ?", userID, tagID).
		Offset(offset).Limit(ps).Find(&members)

	mids := make([]int64, 0, len(members))
	for _, m := range members {
		mids = append(mids, m.FollowMid)
	}

	var follows []model.Following
	if len(mids) > 0 {
		database.DB.Where("user_id = ? AND mid IN ?", userID, mids).Find(&follows)
	}

	midMap := map[int64]model.Following{}
	for _, f := range follows {
		midMap[f.Mid] = f
	}

	list := make([]map[string]interface{}, 0, len(mids))
	for _, mid := range mids {
		if f, ok := midMap[mid]; ok {
			list = append(list, f.ToBiliJSON())
		}
	}

	// Client expects data to be a plain array
	response.Success(c, list)
}

// ---------------------------------------------------------------------------
// POST /x/relation/tag/create
// ---------------------------------------------------------------------------

func CreateFollowTag(c *gin.Context) {
	userID := middleware.GetUserID(c)

	tagName := c.PostForm("tag")
	if tagName == "" {
		response.BadRequest(c, "tag is required")
		return
	}

	tagID := nextTagID(userID)
	tag := model.FollowTag{
		UserID: userID,
		TagID:  tagID,
		Name:   tagName,
	}
	database.DB.Create(&tag)

	response.Success(c, tag.ToBiliJSON())
}

// ---------------------------------------------------------------------------
// POST /x/relation/tag/update
// ---------------------------------------------------------------------------

func UpdateFollowTag(c *gin.Context) {
	userID := middleware.GetUserID(c)

	tagIDStr := c.PostForm("tagid")
	tagID, _ := strconv.ParseInt(tagIDStr, 10, 64)
	name := c.PostForm("name")

	if tagID == 0 || name == "" {
		response.BadRequest(c, "tagid and name are required")
		return
	}

	database.DB.Model(&model.FollowTag{}).
		Where("user_id = ? AND tag_id = ?", userID, tagID).
		Update("name", name)

	response.Success(c, nil)
}

// ---------------------------------------------------------------------------
// POST /x/relation/tag/del
// ---------------------------------------------------------------------------

func DelFollowTag(c *gin.Context) {
	userID := middleware.GetUserID(c)

	tagIDStr := c.PostForm("tagid")
	tagID, _ := strconv.ParseInt(tagIDStr, 10, 64)
	if tagID == 0 {
		response.BadRequest(c, "tagid is required")
		return
	}

	database.DB.Where("user_id = ? AND tag_id = ?", userID, tagID).
		Delete(&model.FollowTagMember{})
	database.DB.Where("user_id = ? AND tag_id = ?", userID, tagID).
		Delete(&model.FollowTag{})

	response.Success(c, nil)
}

// ---------------------------------------------------------------------------
// POST /x/relation/tags/addUsers
// ---------------------------------------------------------------------------

func AddUsersToTag(c *gin.Context) {
	userID := middleware.GetUserID(c)

	fidsStr := c.PostForm("fids")
	tagidsStr := c.PostForm("tagids")

	if fidsStr == "" || tagidsStr == "" {
		response.BadRequest(c, "fids and tagids are required")
		return
	}

	fids := parseIntList(fidsStr)
	tagids := parseIntList(tagidsStr)

	for _, tagID := range tagids {
		for _, fid := range fids {
			var existing model.FollowTagMember
			if database.DB.Where("user_id = ? AND tag_id = ? AND follow_mid = ?", userID, tagID, fid).
				First(&existing).Error == gorm.ErrRecordNotFound {
				database.DB.Create(&model.FollowTagMember{
					UserID:    userID,
					TagID:     tagID,
					FollowMid: fid,
				})
			}
		}
		refreshTagCount(userID, tagID)
	}

	response.Success(c, nil)
}

// ---------------------------------------------------------------------------
// POST /x/relation/tag/special/add
// ---------------------------------------------------------------------------

func AddSpecial(c *gin.Context) {
	userID := middleware.GetUserID(c)

	fidStr := c.PostForm("fid")
	fid, _ := strconv.ParseInt(fidStr, 10, 64)
	if fid == 0 {
		response.BadRequest(c, "fid is required")
		return
	}

	database.DB.Model(&model.Following{}).
		Where("user_id = ? AND mid = ?", userID, fid).
		Update("is_special", 1)

	response.Success(c, nil)
}

// ---------------------------------------------------------------------------
// POST /x/relation/tag/special/del
// ---------------------------------------------------------------------------

func DelSpecial(c *gin.Context) {
	userID := middleware.GetUserID(c)

	fidStr := c.PostForm("fid")
	fid, _ := strconv.ParseInt(fidStr, 10, 64)
	if fid == 0 {
		response.BadRequest(c, "fid is required")
		return
	}

	database.DB.Model(&model.Following{}).
		Where("user_id = ? AND mid = ?", userID, fid).
		Update("is_special", 0)

	response.Success(c, nil)
}

// ===========================================================================
// Phase 4 – Bangumi Follow
// ===========================================================================

// ---------------------------------------------------------------------------
// GET /x/space/bangumi/follow/list
// ---------------------------------------------------------------------------

func BangumiFollowList(c *gin.Context) {
	userID := middleware.GetUserID(c)

	pn, _ := strconv.Atoi(c.DefaultQuery("pn", "1"))
	ps := 15 // Bilibili default for bangumi
	typeStr := c.DefaultQuery("type", "1")
	seasonType, _ := strconv.Atoi(typeStr)
	followStatusStr := c.DefaultQuery("follow_status", "0")
	followStatus, _ := strconv.Atoi(followStatusStr)
	offset := (pn - 1) * ps

	query := database.DB.Model(&model.BangumiFollow{}).Where("user_id = ?", userID)
	if seasonType > 0 {
		query = query.Where("season_type = ?", seasonType)
	}
	if followStatus > 0 {
		query = query.Where("follow_status = ?", followStatus)
	}

	var total int64
	query.Count(&total)

	var items []model.BangumiFollow
	query.Order("follow_time DESC").Offset(offset).Limit(ps).Find(&items)

	list := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		list = append(list, item.ToBiliJSON())
	}

	response.Success(c, gin.H{
		"list":      list,
		"pn":        pn,
		"ps":        ps,
		"total":     total,
		"has_next":  int64(offset+ps) < total,
	})
}

// ---------------------------------------------------------------------------
// POST /pgc/web/follow/add
// ---------------------------------------------------------------------------

func PgcAdd(c *gin.Context) {
	userID := middleware.GetUserID(c)

	seasonIDStr := c.PostForm("season_id")
	seasonID, _ := strconv.ParseInt(seasonIDStr, 10, 64)
	if seasonID == 0 {
		response.BadRequest(c, "season_id is required")
		return
	}

	now := time.Now().Unix()

	var existing model.BangumiFollow
	if database.DB.Where("user_id = ? AND season_id = ?", userID, seasonID).
		First(&existing).Error == gorm.ErrRecordNotFound {
		b := model.BangumiFollow{
			UserID:       userID,
			SeasonID:     seasonID,
			FollowStatus: 1, // "want to watch" by default
			FollowTime:   now,
		}
		database.DB.Create(&b)
	}

	response.PgcSuccess(c, gin.H{
		"toast": "追番成功",
	})
}

// ---------------------------------------------------------------------------
// POST /pgc/web/follow/del
// ---------------------------------------------------------------------------

func PgcDel(c *gin.Context) {
	userID := middleware.GetUserID(c)

	seasonIDStr := c.PostForm("season_id")
	seasonID, _ := strconv.ParseInt(seasonIDStr, 10, 64)
	if seasonID == 0 {
		response.BadRequest(c, "season_id is required")
		return
	}

	database.DB.Where("user_id = ? AND season_id = ?", userID, seasonID).
		Delete(&model.BangumiFollow{})

	response.PgcSuccess(c, gin.H{
		"toast": "已取消追番",
	})
}

// ---------------------------------------------------------------------------
// POST /pgc/web/follow/status/update
// ---------------------------------------------------------------------------

func PgcUpdate(c *gin.Context) {
	userID := middleware.GetUserID(c)

	seasonIDStr := c.PostForm("season_id")
	statusStr := c.PostForm("status")
	seasonID, _ := strconv.ParseInt(seasonIDStr, 10, 64)
	status, _ := strconv.Atoi(statusStr)

	if seasonID == 0 {
		response.BadRequest(c, "season_id is required")
		return
	}

	database.DB.Model(&model.BangumiFollow{}).
		Where("user_id = ? AND season_id = ?", userID, seasonID).
		Update("follow_status", status)

	response.PgcSuccess(c, gin.H{
		"toast": "状态更新成功",
	})
}

// Ensure parseIntList is not redeclared — it's already in favorite.go.
// We use the existing parseIntList from favorite.go since both files
// are in the same package.
var _ = strings.TrimSpace // use strings import
