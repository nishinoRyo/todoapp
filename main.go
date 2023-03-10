package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	"bksn-spm/todoapp/crypto"

	"github.com/joho/godotenv"
	"gorm.io/driver/mysql"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"

	. "bksn-spm/todoapp/SessionInfo"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/joho/godotenv/autoload"
	"gorm.io/gorm"
)

var LoginInfo SessionInfo

type Todo struct {
	gorm.Model
	Text   string
	Status string
}

type User struct {
	gorm.Model
	Username string `form:"username" binding:"required" gorm:"unique;not null"`
	Password string `form:"password" binding:"required"`
}

//DB初期化
func dbInit() *gorm.DB {

	dbUser := os.Getenv("DB_USER")                      // e.g. 'my-db-user'
	dbPwd := os.Getenv("DB_PASS")                       // e.g. 'my-db-password'
	dbName := os.Getenv("DB_NAME")                      // e.g. 'my-database'
	unixSocketPath := os.Getenv("INSTANCE_UNIX_SOCKET") // e.g. '/cloudsql/project:region:instance'

	dbURI := fmt.Sprintf("%s:%s@unix(/cloudsql/%s)/%s?parseTime=true",
		dbUser, dbPwd, unixSocketPath, dbName)

	log.Println("dbUser:" + dbUser)
	log.Println("dbPwd:" + dbPwd)
	log.Println("unixSocketPath:" + unixSocketPath)
	log.Println("dbName:" + dbName)
	log.Println("dbURI:" + dbURI)

	db, err := gorm.Open(mysql.Open(dbURI), &gorm.Config{})

	if err != nil {
		panic("データベース開けず（dbInit）")
	}

	db.AutoMigrate(&Todo{})

	return db

}

// DB追加
func dbInsert(text string, status string) {

	db := dbInit()

	db.Create(&Todo{Text: text, Status: status})
}

// DB更新
func dbUpdate(id int, text string, status string) {

	db := dbInit()

	var todo Todo
	db.First(&todo, id)
	todo.Text = text
	todo.Status = status
	db.Save(&todo)

}

// DB削除
func dbDelete(id int) {

	db := dbInit()

	var todo Todo
	db.First(&todo, id)
	db.Delete(&todo)

}

//DB全取得
func dbGetAll() []Todo {

	db := dbInit()

	var todos []Todo
	db.Order("created_at desc").Find(&todos)

	return todos
}

// DB1つ取得
func dbGetOne(id int) Todo {

	db := dbInit()

	var todo Todo
	db.First(&todo, id)
	return todo
}

func main() {
	router := gin.Default()
	router.LoadHTMLGlob("templates/*.html")

	// セッションの設定
	store := cookie.NewStore([]byte("secret"))
	router.Use(sessions.Sessions("mysession", store))

	menu := router.Group("/user")
	menu.Use(sessionCheck())
	{
		//Index
		menu.GET("/", func(ctx *gin.Context) {
			todos := dbGetAll()
			ctx.HTML(200, "index.html", gin.H{"todos": todos})
		})

		//Create
		menu.POST("/new", func(ctx *gin.Context) {
			text := ctx.PostForm("text")
			status := ctx.PostForm("status")
			dbInsert(text, status)
			ctx.Redirect(302, "/")
		})

		//Detail
		menu.GET("/detail/:id", func(ctx *gin.Context) {
			n := ctx.Param("id")
			id, err := strconv.Atoi(n)
			if err != nil {
				panic(err)
			}
			todo := dbGetOne(id)
			ctx.HTML(200, "detail.html", gin.H{"todo": todo})
		})

		//Update
		menu.POST("/update/:id", func(ctx *gin.Context) {
			n := ctx.Param("id")
			id, err := strconv.Atoi(n)
			if err != nil {
				panic("ERROR")
			}
			text := ctx.PostForm("text")
			status := ctx.PostForm("status")
			dbUpdate(id, text, status)
			ctx.Redirect(302, "/")
		})

		//削除確認
		menu.GET("/delete_check/:id", func(ctx *gin.Context) {
			n := ctx.Param("id")
			id, err := strconv.Atoi(n)
			if err != nil {
				panic("ERROR")
			}
			todo := dbGetOne(id)
			ctx.HTML(200, "delete.html", gin.H{"todo": todo})
		})

		//Delete
		menu.POST("/delete/:id", func(ctx *gin.Context) {
			n := ctx.Param("id")
			id, err := strconv.Atoi(n)
			if err != nil {
				panic("ERROR")
			}
			dbDelete(id)
			ctx.Redirect(302, "/")

		})
	}

	// ユーザー登録画面
	router.GET("/signup", func(c *gin.Context) {

		c.HTML(200, "signup.html", gin.H{})
	})

	// ユーザー登録
	router.POST("/signup", func(c *gin.Context) {
		var form User
		// バリデーション処理
		if err := c.Bind(&form); err != nil {
			c.HTML(http.StatusBadRequest, "signup.html", gin.H{"err": err})
			c.Abort()
		} else {
			username := c.PostForm("username")
			password := c.PostForm("password")
			// 登録ユーザーが重複していた場合にはじく処理
			if err := createUser(username, password); err != nil {
				log.Println("登録ユーザーが重複していた場合にはじく処理")
				c.HTML(http.StatusBadRequest, "signup.html", gin.H{"err": err})
			}
			c.Redirect(302, "/")
		}
	})

	// ユーザーログイン
	router.POST("/login", func(c *gin.Context) {

		// DBから取得したユーザーパスワード(Hash)
		dbPassword := getUser(c.PostForm("username")).Password
		log.Println(dbPassword)
		// フォームから取得したユーザーパスワード
		formPassword := c.PostForm("password")

		// ユーザーパスワードの比較
		if err := crypto.CompareHashAndPassword(dbPassword, formPassword); err != nil {
			log.Println("ログインできませんでした")
			c.HTML(http.StatusBadRequest, "login.html", gin.H{"err": err})
			c.Abort()
		} else {
			log.Println("ログインできました")
			c.Redirect(302, "/")
		}
	})

	router.Run()
}

func sessionCheck() gin.HandlerFunc {
	return func(c *gin.Context) {

		session := sessions.Default(c)
		LoginInfo.username = session.Get("username")

		// セッションがない場合、ログインフォームをだす
		if LoginInfo.username == nil {
			log.Println("ログインしていません")
			c.Redirect(http.StatusMovedPermanently, "/login")
			c.Abort() // これがないと続けて処理されてしまう
		} else {
			c.Set("username", LoginInfo.username) // usernameをセット
			c.Next()
		}
		log.Println("ログインチェック終わり")
	}
}

func createUser(username string, password string) error {
	passwordEncrypt, _ := crypto.PasswordEncrypt(password)
	db := gormConnect()

	// Insert処理
	if err := db.Create(&User{Username: username, Password: passwordEncrypt}).Error; err != nil {
		return err
	}

	return nil
}

func gormConnect() *gorm.DB {
	err := godotenv.Load()
	if err != nil {
		log.Fatal(err)
	}
	dbUser := os.Getenv("DB_USER")                      // e.g. 'my-db-user'
	dbPwd := os.Getenv("DB_PASS")                       // e.g. 'my-db-password'
	dbName := os.Getenv("DB_NAME")                      // e.g. 'my-database'
	unixSocketPath := os.Getenv("INSTANCE_UNIX_SOCKET") // e.g. '/cloudsql/project:region:instance'

	dbURI := fmt.Sprintf("%s:%s@unix(/cloudsql/%s)/%s?parseTime=true",
		dbUser, dbPwd, unixSocketPath, dbName)
	db, err := gorm.Open(mysql.Open(dbURI), &gorm.Config{})

	if err != nil {
		panic(err.Error())
	}

	return db
}

// ユーザーを一件取得
func getUser(username string) User {
	db := gormConnect()
	var user User
	db.First(&user, "username = ?", username)
	return user
}
