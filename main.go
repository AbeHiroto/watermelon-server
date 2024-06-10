package main

import (
	"net/http"
	"time"

	"go.uber.org/zap"

	"xicserver/database" //PostgreSQLとRedisの初期化
	"xicserver/handlers" //Websocket接続へのアップグレードとホーム画面での構成に必要な情報の取得
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
	// ロガーの初期化とクリーンナップ
	logger, err = utils.InitLogger()
	if err != nil {
		panic(err)
	}
	defer logger.Sync()

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
		// 開発環境でのみ設定ファイルを使用
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
		AllowOrigins:     []string{"*"}, //ここにデプロイサーバーのIPアドレスを設定、"http://localhost:*"
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// OPTIONSリクエストを処理するハンドラ
	router.OPTIONS("/*path", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	//各HTTPリクエストのルーティング
	router.POST("/create", func(c *gin.Context) {
		screens.NewGame(c, db, logger)
	})
	router.GET("/home", func(c *gin.Context) {
		handlers.HomeHandler(c, db, logger)
	})
	router.GET("/room/info", func(c *gin.Context) {
		screens.MyRoomInfo(c, db, logger)
	})
	router.PUT("/request/reply", func(c *gin.Context) {
		screens.ReplyHandler(c, db, logger)
	})
	router.DELETE("/room", func(c *gin.Context) {
		screens.DeleteMyRoom(c, db, logger)
	})
	router.GET("/play/:uniqueToken", func(c *gin.Context) {
		screens.GetRoomInfo(c, db, logger)
	})
	router.POST("/challenger/create/:uniqueToken", func(c *gin.Context) {
		screens.NewChallenge(c, db, logger)
	})
	router.GET("/request/info", func(c *gin.Context) {
		screens.MyRequestInfo(c, db, logger)
	})
	router.DELETE("/request/disable", func(c *gin.Context) {
		screens.DisableMyRequest(c, db, logger)
	})
	router.GET("/ws", func(c *gin.Context) {
		handlers.WebSocketConnections(c.Request.Context(), c.Writer, c.Request, db, rdb, logger, clients, games, upgrader)
	})

	// テスト時はHTTPサーバーとして運用。デフォルトポートは ":8080"
	router.Run()

	// // 本番環境ではコメントアウトを解除し、HTTPSサーバーとして運用
	// err = router.RunTLS(":443", "path/to/cert.pem", "path/to/key.pem")
	// if err != nil {
	// 	logger.Fatal("Failed to run HTTPS server: ", zap.Error(err))
	// }
}
