package main

import (
	"log"

	"github.com/gin-gonic/gin"

	"piliminusb/config"
	"piliminusb/database"
	"piliminusb/handler"
	"piliminusb/middleware"
	"piliminusb/model"
)

func main() {
	cfg := config.Get()

	// Database
	database.Init()
	database.DB.AutoMigrate(&model.User{}, &model.WatchLater{}, &model.WatchHistory{}, &model.UserSettings{}, &model.FavFolder{}, &model.FavResource{}, &model.Following{}, &model.FollowTag{}, &model.FollowTagMember{}, &model.BangumiFollow{})

	// Router
	r := gin.Default()

	// Public routes
	auth := r.Group("/auth")
	{
		auth.POST("/register", handler.Register)
		auth.POST("/login", handler.Login)
	}

	// Protected routes (all future Phase 1-4 endpoints go here)
	api := r.Group("/")
	api.Use(middleware.Auth())
	{
		// Phase 1: Watch Later
		api.GET("/x/v2/history/toview/web", handler.ToviewList)
		api.POST("/x/v2/history/toview/add", handler.ToviewAdd)
		api.POST("/x/v2/history/toview/v2/dels", handler.ToviewDel)
		api.POST("/x/v2/history/toview/clear", handler.ToviewClear)
		api.GET("/x/v2/medialist/resource/list", handler.MediaList)

		// Phase 2: History
		api.GET("/x/web-interface/history/cursor", handler.HistoryList)
		api.GET("/x/web-interface/history/search", handler.SearchHistory)
		api.POST("/x/v2/history/delete", handler.DelHistory)
		api.POST("/x/v2/history/clear", handler.ClearHistory)
		api.POST("/x/v2/history/shadow/set", handler.HistoryShadowSet)
		api.GET("/x/v2/history/shadow", handler.HistoryShadow)
		api.POST("/x/click-interface/web/heartbeat", handler.HeartBeat)
		api.POST("/x/v2/history/report", handler.HistoryReport)
		api.POST("/x/v1/medialist/history", handler.MedialistHistory)
		api.GET("/x/v2/history/progress", handler.HistoryProgress)

		// Phase 3: Favorites — Folder Management
		api.GET("/x/v3/fav/folder/created/list-all", handler.AllFavFolders)
		api.GET("/x/v3/fav/folder/created/list", handler.ListFavFolders)
		api.GET("/x/v3/fav/folder/info", handler.FavFolderInfo)
		api.POST("/x/v3/fav/folder/add", handler.AddFavFolder)
		api.POST("/x/v3/fav/folder/edit", handler.EditFavFolder)
		api.POST("/x/v3/fav/folder/del", handler.DelFavFolder)
		api.POST("/x/v3/fav/folder/sort", handler.SortFavFolder)

		// Phase 3: Favorites — Resource Management
		api.GET("/x/v3/fav/resource/list", handler.ListFavResources)
		api.POST("/x/v3/fav/resource/batch-deal", handler.BatchDealFav)
		api.POST("/x/v3/fav/resource/unfav-all", handler.UnfavAll)
		api.POST("/x/v3/fav/resource/copy", handler.CopyFavResource)
		api.POST("/x/v3/fav/resource/move", handler.MoveFavResource)
		api.POST("/x/v3/fav/resource/clean", handler.CleanFavResource)
		api.POST("/x/v3/fav/resource/sort", handler.SortFavResource)

		// Phase 3: Watch Later ↔ Favorites Cross-Operations
		api.POST("/x/v2/history/toview/copy", handler.ToviewCopy)
		api.POST("/x/v2/history/toview/move", handler.ToviewMove)

		// Phase 4: Following Management
		api.GET("/x/relation", handler.Relation)
		api.GET("/x/relation/followings", handler.Followings)
		api.GET("/x/relation/followings/search", handler.FollowingsSearch)
		api.POST("/x/relation/modify", handler.RelationMod)
		api.GET("/x/relation/tags", handler.FollowTags)
		api.GET("/x/relation/tag", handler.FollowTagMembers)
		api.POST("/x/relation/tag/create", handler.CreateFollowTag)
		api.POST("/x/relation/tag/update", handler.UpdateFollowTag)
		api.POST("/x/relation/tag/del", handler.DelFollowTag)
		api.POST("/x/relation/tags/addUsers", handler.AddUsersToTag)
		api.POST("/x/relation/tag/special/add", handler.AddSpecial)
		api.POST("/x/relation/tag/special/del", handler.DelSpecial)

		// Phase 4: Bangumi Follow
		api.GET("/x/space/bangumi/follow/list", handler.BangumiFollowList)
		api.POST("/pgc/web/follow/add", handler.PgcAdd)
		api.POST("/pgc/web/follow/del", handler.PgcDel)
		api.POST("/pgc/web/follow/status/update", handler.PgcUpdate)

		// Phase 5: Dynamics Feed
		api.GET("/x/polymer/web-dynamic/v1/feed/all", handler.DynamicFeed)
		api.GET("/x/polymer/web-dynamic/v1/portal", handler.DynamicPortal)
	}

	log.Printf("PiliMinusB server starting on :%s", cfg.Server.Port)
	r.Run(":" + cfg.Server.Port)
}
