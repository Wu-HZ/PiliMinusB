package model

import "time"

type WatchHistory struct {
	ID         uint      `gorm:"primaryKey" json:"-"`
	UserID     uint      `gorm:"not null;uniqueIndex:idx_hist_user_aid" json:"-"`
	Aid        int64     `gorm:"not null;uniqueIndex:idx_hist_user_aid" json:"aid"`
	Bvid       string    `gorm:"size:20" json:"bvid"`
	Cid        int64     `json:"cid"`
	Epid       int64     `json:"epid"`
	SeasonID   int64     `json:"season_id"`
	Title      string    `gorm:"size:500" json:"title"`
	LongTitle  string    `gorm:"size:500" json:"long_title"`
	Cover      string    `gorm:"size:500" json:"cover"`
	Duration   int       `json:"duration"`
	Progress   int       `gorm:"default:0" json:"progress"` // seconds, -1 = finished
	AuthorMid  int64     `json:"author_mid"`
	AuthorName string    `gorm:"size:100" json:"author_name"`
	AuthorFace string    `gorm:"size:500" json:"author_face"`
	Badge      string    `gorm:"size:50" json:"badge"`
	Kid        string    `gorm:"size:50" json:"kid"`       // business_oid format for deletion
	Business   string    `gorm:"size:30" json:"business"`  // archive/pgc/live/article
	ViewAt     int64     `gorm:"not null;index:idx_hist_view_at" json:"view_at"`
	Videos     int       `json:"videos"`
	Current    string    `gorm:"size:200" json:"current"`
	IsFinish   int       `gorm:"default:0" json:"is_finish"`
	IsFav      int       `gorm:"default:0" json:"is_fav"`
	CreatedAt  time.Time `json:"-"`
	UpdatedAt  time.Time `json:"-"`
}

// ToBiliJSON converts to Bilibili-compatible history list item JSON.
func (h *WatchHistory) ToBiliJSON() map[string]interface{} {
	return map[string]interface{}{
		"title":       h.Title,
		"long_title":  h.LongTitle,
		"cover":       h.Cover,
		"covers":      nil,
		"uri":         "",
		"history": map[string]interface{}{
			"oid":      h.Aid,
			"epid":     h.Epid,
			"bvid":     h.Bvid,
			"page":     1,
			"cid":      h.Cid,
			"part":     "",
			"business": h.Business,
		},
		"videos":      h.Videos,
		"author_name": h.AuthorName,
		"author_face": h.AuthorFace,
		"author_mid":  h.AuthorMid,
		"view_at":     h.ViewAt,
		"progress":    h.Progress,
		"badge":       h.Badge,
		"show_title":  "",
		"duration":    h.Duration,
		"current":     h.Current,
		"total":       0,
		"new_desc":    "",
		"is_finish":   h.IsFinish,
		"is_fav":      h.IsFav,
		"kid":         h.Aid, // used for deletion
		"tag_name":    "",
		"live_status": 0,
	}
}

type UserSettings struct {
	UserID        uint `gorm:"primaryKey" json:"-"`
	HistoryPaused int  `gorm:"default:0" json:"history_paused"` // 0: recording, 1: paused
}
