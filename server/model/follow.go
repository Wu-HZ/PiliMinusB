package model

import "time"

// ---------------------------------------------------------------------------
// Phase 4: Following & Bangumi
// ---------------------------------------------------------------------------

// Following represents a user-followed account (UP主).
type Following struct {
	ID          uint      `gorm:"primaryKey" json:"-"`
	UserID      uint      `gorm:"not null;uniqueIndex:idx_follow_user_mid" json:"-"`
	Mid         int64     `gorm:"not null;uniqueIndex:idx_follow_user_mid" json:"mid"`
	Name        string    `gorm:"size:200" json:"uname"`
	Face        string    `gorm:"size:500" json:"face"`
	Sign        string    `gorm:"size:500" json:"sign"`
	IsSpecial   int       `gorm:"default:0" json:"special"`            // 1 = special follow
	Attribute   int       `gorm:"default:2" json:"attribute"`          // 2 = followed
	MTime       int64     `json:"mtime"`                               // follow timestamp
	OfficialType int      `gorm:"default:-1" json:"-"`
	SortOrder   int       `gorm:"default:0" json:"-"`
	CreatedAt   time.Time `json:"-"`
	UpdatedAt   time.Time `json:"-"`
}

// ToBiliJSON converts to Bilibili-compatible following item JSON.
func (f *Following) ToBiliJSON() map[string]interface{} {
	return map[string]interface{}{
		"mid":            f.Mid,
		"attribute":      f.Attribute,
		"mtime":          f.MTime,
		"special":        f.IsSpecial,
		"uname":          f.Name,
		"face":           f.Face,
		"sign":           f.Sign,
		"face_nft":       0,
		"official_verify": map[string]interface{}{
			"type": f.OfficialType,
			"desc": "",
		},
		"vip": map[string]interface{}{
			"vipType":       0,
			"vipDueDate":    0,
			"vipStatus":     0,
			"themeType":     0,
			"avatar_subscript": 0,
		},
		"nft_icon": "",
		"rec_reason": "",
		"track_id":   "",
	}
}

// FollowTag represents a user-defined grouping of followed accounts.
type FollowTag struct {
	ID        uint      `gorm:"primaryKey" json:"-"`
	UserID    uint      `gorm:"not null;uniqueIndex:idx_ftag_user_tagid" json:"-"`
	TagID     int64     `gorm:"not null;uniqueIndex:idx_ftag_user_tagid" json:"tagid"`
	Name      string    `gorm:"size:100;not null" json:"name"`
	Count     int       `gorm:"default:0" json:"count"`
	Tip       string    `gorm:"size:200;default:''" json:"tip"`
	CreatedAt time.Time `json:"-"`
	UpdatedAt time.Time `json:"-"`
}

// ToBiliJSON converts to Bilibili-compatible tag JSON.
func (t *FollowTag) ToBiliJSON() map[string]interface{} {
	return map[string]interface{}{
		"tagid": t.TagID,
		"name":  t.Name,
		"count": t.Count,
		"tip":   t.Tip,
	}
}

// FollowTagMember maps a following to one or more tags.
type FollowTagMember struct {
	ID          uint      `gorm:"primaryKey" json:"-"`
	UserID      uint      `gorm:"not null;uniqueIndex:idx_ftm_user_tag_mid" json:"-"`
	TagID       int64     `gorm:"not null;uniqueIndex:idx_ftm_user_tag_mid;index:idx_ftm_tag" json:"tagid"`
	FollowMid   int64     `gorm:"not null;uniqueIndex:idx_ftm_user_tag_mid" json:"mid"`
	CreatedAt   time.Time `json:"-"`
	UpdatedAt   time.Time `json:"-"`
}

// BangumiFollow represents a user's followed bangumi/anime/drama.
type BangumiFollow struct {
	ID          uint      `gorm:"primaryKey" json:"-"`
	UserID      uint      `gorm:"not null;uniqueIndex:idx_bangumi_user_season" json:"-"`
	SeasonID    int64     `gorm:"not null;uniqueIndex:idx_bangumi_user_season" json:"season_id"`
	SeasonType  int       `gorm:"default:1" json:"season_type"`   // 1=anime 2=movie 3=documentary 4=guochuang 5=TV 7=variety
	Title       string    `gorm:"size:300" json:"title"`
	Cover       string    `gorm:"size:500" json:"cover"`
	TotalCount  int       `json:"total_count"`
	NewEpDesc   string    `gorm:"size:200" json:"new_ep_desc"`
	NewEpID     int64     `json:"new_ep_id"`
	FollowStatus int      `gorm:"default:0" json:"follow_status"` // 0=not set 1=want 2=watching 3=watched
	Progress    string    `gorm:"size:100" json:"progress"`
	Areas       string    `gorm:"size:200" json:"areas"`
	FollowTime  int64     `json:"follow_time"`
	SortOrder   int       `gorm:"default:0" json:"-"`
	CreatedAt   time.Time `json:"-"`
	UpdatedAt   time.Time `json:"-"`
}

// ToBiliJSON converts to Bilibili-compatible bangumi follow JSON.
func (b *BangumiFollow) ToBiliJSON() map[string]interface{} {
	return map[string]interface{}{
		"season_id":    b.SeasonID,
		"media_id":     0,
		"season_type":  b.SeasonType,
		"season_type_name": seasonTypeName(b.SeasonType),
		"title":        b.Title,
		"cover":        b.Cover,
		"total_count":  b.TotalCount,
		"badge":        "",
		"badge_type":   0,
		"follow_status": b.FollowStatus,
		"is_finish":    0,
		"progress":     b.Progress,
		"new_ep": map[string]interface{}{
			"id":         b.NewEpID,
			"index_show": b.NewEpDesc,
			"cover":      "",
			"title":      "",
			"long_title": "",
			"pub_time":   "",
			"duration":   0,
		},
		"areas": []map[string]interface{}{
			{"name": b.Areas},
		},
		"square_cover":  "",
		"first_ep":      0,
		"url":           "",
		"subtitle":      "",
	}
}

func seasonTypeName(t int) string {
	switch t {
	case 1:
		return "番剧"
	case 2:
		return "电影"
	case 3:
		return "纪录片"
	case 4:
		return "国创"
	case 5:
		return "电视剧"
	case 7:
		return "综艺"
	default:
		return ""
	}
}
