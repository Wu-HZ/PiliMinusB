package model

import "time"

type WatchLater struct {
	ID        uint      `gorm:"primaryKey" json:"-"`
	UserID    uint      `gorm:"not null;uniqueIndex:idx_user_aid" json:"-"`
	Aid       int64     `gorm:"not null;uniqueIndex:idx_user_aid" json:"aid"`
	Bvid      string    `gorm:"size:20;not null" json:"bvid"`
	Title     string    `gorm:"size:500" json:"title"`
	Pic       string    `gorm:"size:500" json:"pic"`
	Duration  int       `json:"duration"`
	OwnerMid  int64     `json:"owner_mid"`
	OwnerName string    `gorm:"size:100" json:"owner_name"`
	OwnerFace string    `gorm:"size:500" json:"owner_face"`
	Videos    int       `json:"videos"`
	Cid       int64     `json:"cid"`
	Pubdate   int64     `json:"pubdate"`
	Progress  int       `gorm:"default:0" json:"progress"`
	Viewed    int       `gorm:"default:0" json:"viewed"` // 0: unwatched, 1: watched
	AddedAt   int64     `gorm:"not null" json:"added_at"`
	CreatedAt time.Time `json:"-"`
	UpdatedAt time.Time `json:"-"`
}

// ToMediaListJSON converts the model to MediaList-compatible JSON map.
// This matches the format expected by MediaListItemModel.fromJson on the client.
func (w *WatchLater) ToMediaListJSON() map[string]interface{} {
	return map[string]interface{}{
		"id":       w.Aid,
		"title":    w.Title,
		"cover":    w.Pic,
		"duration": w.Duration,
		"pubtime":  w.Pubdate,
		"bv_id":    w.Bvid,
		"type":     2, // ugc
		"upper": map[string]interface{}{
			"mid":  w.OwnerMid,
			"name": w.OwnerName,
			"face": w.OwnerFace,
		},
		"cnt_info": map[string]interface{}{
			"play":    0,
			"danmaku": 0,
		},
		"pages": []map[string]interface{}{
			{"id": w.Cid, "title": "", "page": 1},
		},
	}
}

// ToBiliJSON converts the model to Bilibili-compatible JSON map.
func (w *WatchLater) ToBiliJSON() map[string]interface{} {
	return map[string]interface{}{
		"aid":      w.Aid,
		"bvid":     w.Bvid,
		"title":    w.Title,
		"pic":      w.Pic,
		"duration": w.Duration,
		"pubdate":  w.Pubdate,
		"cid":      w.Cid,
		"progress": w.Progress,
		"videos":   w.Videos,
		"owner": map[string]interface{}{
			"mid":  w.OwnerMid,
			"name": w.OwnerName,
			"face": w.OwnerFace,
		},
		"stat": map[string]interface{}{
			"aid":    w.Aid,
			"view":   0,
			"danmaku": 0,
		},
		"pages": []map[string]interface{}{
			{"cid": w.Cid, "page": 1, "part": ""},
		},
		"is_pgc":      false,
		"pgc_label":   "",
		"is_pugv":     false,
		"season_id":   0,
		"redirect_url": "",
	}
}
