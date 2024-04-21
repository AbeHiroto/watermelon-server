package main

import (
	"net/http"
	"time"

	"go.uber.org/zap"

	"xicserver/bribe"    //BRIBEのゲームロジック
	"xicserver/database" //PostgreSQLとRedisの初期化
	"xicserver/models"   //モデル定義
	"xicserver/screens"  //フロントの画面構成やマッチングに関連するHTTPリクエストの処理
	"xicserver/utils"    //ロガーの初期化とCronジョブ(PostgreSQLの定期クリーンナップ)

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/gorilla/websocket"
	"gorm.io/gorm"
)

func main() {
	var logger *zap.Logger
	var err error
	logger, err = utils.InitLogger() // ロガーの初期化
	if err != nil {
		panic(err) // 失敗した場合はプログラム停止
	}
	defer logger.Sync() // ロガーのクリーンアップ

	// Websocket接続で用いる変数を初期化
	clients := make(map[*models.Client]bool)
	games := make(map[uint]*models.Game)
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	// 非同期でPostgreSQLとRedisの初期化
	var db *gorm.DB
	var rdb *redis.Client
	done := make(chan bool)

	go func() {
		config, err := database.LoadConfig("config.json")
		if err != nil {
			logger.Fatal("設定ファイルの読み込みに失敗しました", zap.Error(err))
		}
		db, err = database.InitPostgreSQL(config, logger)
		if err != nil {
			logger.Fatal("PostgreSQLの初期化に失敗しました", zap.Error(err))
		}
		done <- true
	}()

	go func() {
		rdb, err = database.InitRedis(logger)
		if err != nil {
			logger.Fatal("Failed to initialize Redis", zap.Error(err))
		}
		done <- true
	}()

	// 2つの初期化が完了するのを待つ
	<-done
	<-done

	// クーロンスケジューラのセットアップと呼び出し
	go utils.CronCleaner(db, logger)

	router := gin.Default()
	// dbとrdbを全てのリクエストで利用できるようにする
	router.Use(func(c *gin.Context) {
		c.Set("db", db)
		c.Set("rdb", rdb)
		c.Next()
	})
	//リクエストロガーを起動
	router.Use(gin.Recovery(), utils.RequestLogger(logger))

	//CORS（Cross-Origin Resource Sharing）ポリシーを設定
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://192.168.1.1:8080"}, //ここにデプロイサーバーのIPアドレスを設定
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	//各HTTPリクエストのルーティング
	router.POST("/create", func(c *gin.Context) {
		screens.RoomCreate(c, db, logger)
	})
	router.GET("/home", func(c *gin.Context) {
		screens.HomeHandler(c, db, logger)
	})
	router.GET("/room/info", func(c *gin.Context) {
		screens.MyRoomInfoHandler(c, db, logger)
	})
	router.PUT("/request/reply", func(c *gin.Context) {
		screens.ReplyHandler(c, db, logger)
	})
	router.DELETE("/room", func(c *gin.Context) {
		screens.RoomDeleteHandler(c, db, logger)
	})
	router.GET("/request/info", func(c *gin.Context) {
		screens.MyRequestHandler(c, db, logger)
	})
	router.DELETE("/request/disable", func(c *gin.Context) {
		screens.DisableMyRequest(c, db, logger)
	})
	router.POST("/challenger/create/:uniqueToken", func(c *gin.Context) {
		screens.ChallengerHandler(c, db, logger)
	})
	router.GET("/ws", func(c *gin.Context) {
		bribe.HandleConnections(c.Request.Context(), c.Writer, c.Request, db, rdb, logger, clients, games, upgrader)
	})

	// テスト時はHTTPサーバーとして運用。デフォルトポートは ":8080"
	router.Run()

	// // 本番環境ではコメントアウトを解除し、HTTPSサーバーとして運用
	// err = router.RunTLS(":443", "path/to/cert.pem", "path/to/key.pem")
	// if err != nil {
	// 	logger.Fatal("Failed to run HTTPS server: ", zap.Error(err))
	// }
}
