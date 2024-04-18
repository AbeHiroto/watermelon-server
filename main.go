package main

import (
	"go.uber.org/zap"

	"xicserver/bribe/websocket" //BRIBEの実際のゲームロジック
	"xicserver/database"
	"xicserver/handlers" //フロントの画面構成やマッチングに関連するHTTPリクエストの処理
	"xicserver/utils"

	"github.com/gin-gonic/gin"
)

func main() {
	// ロガーの初期化
	logger, err := utils.InitLogger()
	if err != nil {
		panic(err) // ロガーの初期化に失敗した場合はプログラムを停止
	}
	defer logger.Sync() // ロガーのクリーンアップ

	// 開発用設定ファイル"config.json"の読み込み
	config, err := database.LoadConfig("config.json")
	if err != nil {
		logger.Fatal("設定ファイルの読み込みに失敗しました", zap.Error(err))
	}

	// PostgreSQLの初期化
	db, err := database.InitDB(config, logger)
	if err != nil {
		logger.Fatal("データベースの初期化に失敗しました", zap.Error(err))
	}
	//defer db.Close() // データベース接続のクローズ（GORMv2からは不要）

	// Redisの初期化
	rdb, err := database.InitializeRedis(logger)
	if err != nil {
		logger.Fatal("Failed to initialize Redis", zap.Error(err))
	}

	// スケジューラのセットアップと関数の呼び出し
	utils.Cleaner(db, logger)

	router := gin.Default()
	// Ginのミドルウェアを使用して、dbとrdbを全てのリクエストで利用できるようにする
	router.Use(func(c *gin.Context) {
		c.Set("db", db)
		c.Set("rdb", rdb)
		c.Next()
	})
	router.Use(gin.Recovery(), utils.RequestLogger(logger))

	//各HTTPリクエストのルーティング
	router.POST("/create", func(c *gin.Context) {
		handlers.RoomCreate(c, db, logger)
	})
	router.GET("/home", func(c *gin.Context) {
		handlers.HomeHandler(c, db, logger)
	})
	router.GET("/room/info", func(c *gin.Context) {
		handlers.MyRoomInfoHandler(c, db, logger)
	})
	router.PUT("/request/reply", func(c *gin.Context) {
		handlers.ReplyHandler(c, db, logger)
	})
	router.DELETE("/room", func(c *gin.Context) {
		handlers.RoomDeleteHandler(c, db, logger)
	})
	router.GET("/request/info", func(c *gin.Context) {
		handlers.MyRequestHandler(c, db, logger)
	})
	router.DELETE("/request/disable", func(c *gin.Context) {
		handlers.DisableMyRequest(c, db, logger)
	})
	router.POST("/challenger/create/:uniqueToken", func(c *gin.Context) {
		handlers.ChallengerHandler(c, db, logger)
	})
	router.GET("/ws", func(c *gin.Context) {
		websocket.HandleConnections(c.Request.Context(), c.Writer, c.Request, db, rdb, logger)
	})

	// テスト時はHTTPサーバーとして運用。デフォルトポートは ":8080"
	router.Run()

	// // 本番環境ではコメントアウトを解除し、HTTPSサーバーとして運用
	// err = router.RunTLS(":443", "path/to/cert.pem", "path/to/key.pem")
	// if err != nil {
	// 	logger.Fatal("Failed to run HTTPS server: ", zap.Error(err))
	// }
}
