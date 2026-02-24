package handler

import (
	"fmt"
	"log"
	"math"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"piliminusb/bilibili"
	"piliminusb/database"
	"piliminusb/middleware"
	"piliminusb/model"
	"piliminusb/response"
)

// ===========================================================================
// Phase 5 – Dynamics Feed (video-only, from followed UPs)
// ===========================================================================

// ---------------------------------------------------------------------------
// GET /x/polymer/web-dynamic/v1/portal
// ---------------------------------------------------------------------------

func DynamicPortal(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var follows []model.Following
	database.DB.Where("user_id = ?", userID).Order("m_time DESC").Find(&follows)

	items := make([]gin.H, 0, len(follows))
	for _, f := range follows {
		items = append(items, gin.H{
			"mid":        f.Mid,
			"uname":      f.Name,
			"face":       f.Face,
			"has_update": false,
		})
	}

	response.Success(c, gin.H{
		"up_list": gin.H{
			"items":    items,
			"has_more": false,
		},
		"live_users": gin.H{
			"count": 0,
			"items": []gin.H{},
		},
	})
}

// ---------------------------------------------------------------------------
// GET /x/polymer/web-dynamic/v1/feed/all
// ---------------------------------------------------------------------------

func DynamicFeed(c *gin.Context) {
	userID := middleware.GetUserID(c)

	offsetStr := c.Query("offset")
	hostMidStr := c.Query("host_mid")

	var follows []model.Following
	if hostMidStr != "" {
		mid, _ := strconv.ParseInt(hostMidStr, 10, 64)
		if mid > 0 {
			database.DB.Where("user_id = ? AND mid = ?", userID, mid).Find(&follows)
		}
	} else {
		database.DB.Where("user_id = ?", userID).Find(&follows)
	}

	if len(follows) == 0 {
		log.Printf("[dynamic] DynamicFeed userID=%d: no followings found in DB", userID)
		response.Success(c, gin.H{
			"has_more":    false,
			"offset":      "",
			"update_num":  0,
			"items":       []gin.H{},
		})
		return
	}

	log.Printf("[dynamic] DynamicFeed userID=%d: %d followings, offset=%q", userID, len(follows), offsetStr)

	// Build a mid→Following map for author info
	midMap := make(map[int64]*model.Following, len(follows))
	for i := range follows {
		midMap[follows[i].Mid] = &follows[i]
	}

	// Concurrently fetch videos (semaphore = 10)
	type midVideos struct {
		mid    int64
		videos []bilibili.SpaceVideo
	}

	sem := make(chan struct{}, 10)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var allVideos []midVideos

	for _, f := range follows {
		wg.Add(1)
		go func(mid int64) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			vids, _ := bilibili.FetchUserVideos(mid, 10)
			if len(vids) > 0 {
				mu.Lock()
				allVideos = append(allVideos, midVideos{mid: mid, videos: vids})
				mu.Unlock()
			}
		}(f.Mid)
	}
	wg.Wait()

	log.Printf("[dynamic] DynamicFeed userID=%d: fetched videos from %d/%d UPs", userID, len(allVideos), len(follows))

	// Flatten and attach owner mid
	type videoWithOwner struct {
		bilibili.SpaceVideo
		OwnerMid int64
	}
	var flat []videoWithOwner
	for _, mv := range allVideos {
		for _, v := range mv.videos {
			flat = append(flat, videoWithOwner{SpaceVideo: v, OwnerMid: mv.mid})
		}
	}

	// Sort by pubdate DESC
	sort.Slice(flat, func(i, j int) bool {
		return flat[i].Pubdate > flat[j].Pubdate
	})

	// Cursor-based pagination: skip items with pubdate >= offset
	var offsetTS int64
	if offsetStr != "" {
		offsetTS, _ = strconv.ParseInt(offsetStr, 10, 64)
	}

	var page []videoWithOwner
	for _, v := range flat {
		if offsetTS > 0 && v.Pubdate >= offsetTS {
			continue
		}
		page = append(page, v)
		if len(page) >= 20 {
			break
		}
	}

	hasMore := len(page) == 20
	nextOffset := ""
	if hasMore && len(page) > 0 {
		nextOffset = strconv.FormatInt(page[len(page)-1].Pubdate, 10)
	}

	items := make([]gin.H, 0, len(page))
	for _, v := range page {
		owner := midMap[v.OwnerMid]
		ownerName := ""
		ownerFace := ""
		if owner != nil {
			ownerName = owner.Name
			ownerFace = owner.Face
		}

		aidStr := strconv.FormatInt(v.Aid, 10)

		items = append(items, gin.H{
			"id_str":  aidStr,
			"type":    "DYNAMIC_TYPE_AV",
			"visible": true,
			"basic": gin.H{
				"comment_id_str": aidStr,
				"comment_type":   1,
				"rid_str":        aidStr,
			},
			"modules": gin.H{
				"module_author": gin.H{
					"mid":        v.OwnerMid,
					"name":       ownerName,
					"face":       ownerFace,
					"pub_ts":     v.Pubdate,
					"pub_time":   formatPubTime(v.Pubdate),
					"pub_action": "投稿了视频",
					"type":       "AUTHOR_TYPE_NORMAL",
				},
				"module_dynamic": gin.H{
					"major": gin.H{
						"type": "MAJOR_TYPE_ARCHIVE",
						"archive": gin.H{
							"aid":           aidStr,
							"bvid":          v.Bvid,
							"title":         v.Title,
							"cover":         v.Pic,
							"duration_text": formatDuration(v.Duration),
							"jump_url":      fmt.Sprintf("//www.bilibili.com/video/%s", v.Bvid),
							"stat": gin.H{
								"play":    formatCount(v.Play),
								"danmaku": formatCount(v.Danmaku),
							},
							"type": 1,
						},
					},
				},
				"module_stat": gin.H{
					"like":    gin.H{"count": nil},
					"comment": gin.H{"count": nil},
					"forward": gin.H{"count": nil},
				},
			},
		})
	}

	response.Success(c, gin.H{
		"has_more":   hasMore,
		"offset":     nextOffset,
		"update_num": 0,
		"items":      items,
	})
}

// ---------------------------------------------------------------------------
// Helper functions
// ---------------------------------------------------------------------------

func formatDuration(sec int) string {
	if sec <= 0 {
		return "00:00"
	}
	h := sec / 3600
	m := (sec % 3600) / 60
	s := sec % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}

func formatCount(n int64) string {
	if n < 0 {
		return "0"
	}
	if n < 10000 {
		return strconv.FormatInt(n, 10)
	}
	wan := float64(n) / 10000.0
	if wan < 1000 {
		// Round to 1 decimal
		rounded := math.Round(wan*10) / 10
		if rounded == math.Trunc(rounded) {
			return fmt.Sprintf("%d万", int64(rounded))
		}
		return fmt.Sprintf("%.1f万", rounded)
	}
	yi := wan / 10000.0
	rounded := math.Round(yi*10) / 10
	if rounded == math.Trunc(rounded) {
		return fmt.Sprintf("%d亿", int64(rounded))
	}
	return fmt.Sprintf("%.1f亿", rounded)
}

func formatPubTime(ts int64) string {
	t := time.Unix(ts, 0)
	now := time.Now()
	diff := now.Sub(t)

	switch {
	case diff < time.Minute:
		return "刚刚"
	case diff < time.Hour:
		return fmt.Sprintf("%d分钟前", int(diff.Minutes()))
	case diff < 24*time.Hour:
		return fmt.Sprintf("%d小时前", int(diff.Hours()))
	case diff < 2*24*time.Hour:
		return "昨天"
	default:
		if t.Year() == now.Year() {
			return fmt.Sprintf("%d月%d日", t.Month(), t.Day())
		}
		return fmt.Sprintf("%d年%d月%d日", t.Year(), t.Month(), t.Day())
	}
}
