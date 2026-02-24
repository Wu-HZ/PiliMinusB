package model

import "time"

// ---------------------------------------------------------------------------
// Phase 3: Favorites
// ---------------------------------------------------------------------------

// FavFolder represents a user's favorite folder.
type FavFolder struct {
	ID         uint      `gorm:"primaryKey" json:"-"`
	UserID     uint      `gorm:"not null;uniqueIndex:idx_fav_user_media" json:"-"`
	MediaID    int64     `gorm:"not null;uniqueIndex:idx_fav_user_media" json:"media_id"`
	Title      string    `gorm:"size:200;not null" json:"title"`
	Cover      string    `gorm:"size:500;default:''" json:"cover"`
	Intro      string    `gorm:"size:500;default:''" json:"intro"`
	MediaCount int       `gorm:"default:0" json:"media_count"`
	Ctime      int64     `json:"ctime"`
	Mtime      int64     `json:"mtime"`
	SortOrder  int       `gorm:"default:0" json:"sort_order"`
	IsDefault  int       `gorm:"default:0" json:"is_default"`
	CreatedAt  time.Time `json:"-"`
	UpdatedAt  time.Time `json:"-"`
}

// ToBiliJSON converts to Bilibili-compatible folder info JSON.
func (f *FavFolder) ToBiliJSON(ownerMid int64) map[string]interface{} {
	return map[string]interface{}{
		"id":          f.MediaID,
		"fid":         f.MediaID,
		"mid":         ownerMid,
		"attr":        0,
		"title":       f.Title,
		"cover":       f.Cover,
		"upper": map[string]interface{}{
			"mid":  ownerMid,
			"name": "",
			"face": "",
		},
		"cover_type":  0,
		"intro":       f.Intro,
		"ctime":       f.Ctime,
		"mtime":       f.Mtime,
		"state":       0,
		"fav_state":   0,
		"media_count": f.MediaCount,
		"view_count":  0,
		"is_top":      false,
		"type":        0,
	}
}

// ToBiliJSONWithFavState returns folder info with fav_state indicating the
// given resource is in this folder (1) or not (0).
func (f *FavFolder) ToBiliJSONWithFavState(ownerMid int64, favState int) map[string]interface{} {
	m := f.ToBiliJSON(ownerMid)
	m["fav_state"] = favState
	return m
}

// FavResource represents a single resource (video) inside a favorite folder.
type FavResource struct {
	ID           uint      `gorm:"primaryKey" json:"-"`
	UserID       uint      `gorm:"not null;uniqueIndex:idx_favr_user_media_res" json:"-"`
	MediaID      int64     `gorm:"not null;uniqueIndex:idx_favr_user_media_res;index:idx_favr_list" json:"media_id"`
	ResourceID   int64     `gorm:"not null;uniqueIndex:idx_favr_user_media_res" json:"resource_id"`
	ResourceType int       `gorm:"default:2" json:"resource_type"` // 2=video
	Title        string    `gorm:"size:500" json:"title"`
	Cover        string    `gorm:"size:500" json:"cover"`
	Intro        string    `gorm:"size:500" json:"intro"`
	Duration     int       `json:"duration"`
	UpperMid     int64     `json:"upper_mid"`
	UpperName    string    `gorm:"size:100" json:"upper_name"`
	Bvid         string    `gorm:"size:20" json:"bvid"`
	Pubtime      int64     `json:"pubtime"`
	FavTime      int64     `gorm:"not null;index:idx_favr_list" json:"fav_time"`
	SortOrder    int       `gorm:"default:0" json:"sort_order"`
	Cid          int64     `json:"cid"`
	CreatedAt    time.Time `json:"-"`
	UpdatedAt    time.Time `json:"-"`
}

// ToBiliJSON converts to Bilibili-compatible resource item JSON.
func (r *FavResource) ToBiliJSON() map[string]interface{} {
	return map[string]interface{}{
		"id":       r.ResourceID,
		"type":     r.ResourceType,
		"title":    r.Title,
		"cover":    r.Cover,
		"intro":    r.Intro,
		"page":     0,
		"duration": r.Duration,
		"upper": map[string]interface{}{
			"mid":  r.UpperMid,
			"name": r.UpperName,
			"face": "",
		},
		"attr": 0,
		"cnt_info": map[string]interface{}{
			"collect":   0,
			"play":      0,
			"thumb_up":  0,
			"thumb_down": 0,
			"share":     0,
			"reply":     0,
			"danmaku":   0,
			"coin":      0,
		},
		"link":    "",
		"ctime":   r.FavTime,
		"pubtime": r.Pubtime,
		"fav_time": r.FavTime,
		"bvid":    r.Bvid,
		"bv_id":   r.Bvid,
		"ugc": map[string]interface{}{
			"first_cid": r.Cid,
		},
	}
}
