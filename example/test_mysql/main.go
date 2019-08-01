package main

import (
	"github.com/mydeeplike/dbx"
	"fmt"
	"os"
	"time"
)

type User struct {
	Uid        int64     `db:"uid"`
	Gid        int64     `db:"gid"`
	Name       string    `db:"name"`
	CreateDate time.Time `db:"createDate"`
}

func main() {

	var err error
	var db *dbx.DB

	// db, err = dbx.Open("mysql", "root:root@tcp(localhost)/test?parseTime=true&charset=utf8")
	db, err = dbx.Open("mysql", "root@tcp(localhost)/test?parseTime=true&charset=utf8")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	// db 输出信息设置
	db.Stdout = os.Stdout // 默认：将 db 产生的错误信息输出到标准输出
	db.Stderr = dbx.OpenLogFile("./db1_error.log") // 将 db 产生的错误信息输出到指定的文件
	// db.Stdout = ioutil.Discard // 默认：将 db 的输出信息重定向到"黑洞"（不输出执行的 SQL 语句等信息）

	// 参数设置
	db.SetMaxIdleConns(10)
	db.SetMaxOpenConns(10)
	db.SetConnMaxLifetime(time.Second * 5)

	// 创建表
	_, err = db.Exec(`DROP TABLE IF EXISTS user;`)
	_, err = db.Exec(`CREATE TABLE user(
		uid        INT(11) PRIMARY KEY AUTO_INCREMENT,
		gid        INT(11) NOT NULL DEFAULT '0',
		name       TEXT             DEFAULT '',
		createDate DATETIME         DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil {
		panic(err)
	}

	// 开启缓存，可选项，一般只针对小表开启缓存，超过 10w 行，不建议开启！
	db.BindStruct("user", &User{}, true)
	db.EnableCache(true)

	// 插入一条
	u1 := &User{1, 1, "jet", time.Now()}
	_, err = db.Table("user").Insert(u1)
	if err != nil {
		panic(err)
	}

	// 读取一条
	u2 := &User{}
	err = db.Table("user").WherePK(1).One(u2)
	if err != nil {
		panic(err)
	}
	fmt.Printf("%+v\n", u2)

	// 更新一条
	u2.Name = "jet.li"
	_, err = db.Table("user").Update(u2)
	if err != nil {
		panic(err)
	}

	// Where 条件 + 更新
	_, err = db.Table("user").WhereM(dbx.M{{"uid", 1}, {"gid", 1}}).UpdateM(dbx.M{{"Name", "jet.li"}})
	if err != nil {
		panic(err)
	}

	// 删除一条
	_, err = db.Table("user").WherePK(1).Delete()
	if err != nil {
		panic(err)
	}

	// 插入多条
	for i := int64(0); i < 5; i++ {
		u := &User{
			Uid: i,
			Gid: i,
			Name: fmt.Sprintf("name-%v", i),
			CreateDate: time.Now(),
		}
		_, err := db.Table("user").Insert(u)
		if err != nil {
			panic(err)
		}
	}

	// 获取多条
	userList := []*User{}
	err = db.Table("user").Where("uid>?", 1).All(&userList)
	if err != nil {
		panic(err)
	}
	for _, u := range userList {
		fmt.Printf("%+v\n", u)
	}

	// 批量更新
	_, err = db.Table("user").Where("uid>?", 3).UpdateM(dbx.M{{"gid", 10}})
	if err != nil {
		panic(err)
	}

	// 批量删除
	_, err = db.Table("user").Where("uid>?", 3).Delete()
	if err != nil {
		panic(err)
	}

	// 总数
	n, err := db.Table("user").Where("uid>?", -1).Count()
	if err != nil {
		panic(err)
	}
	fmt.Printf("count: %v\n", n)

	// 求和
	n, err = db.Table("user").Where("uid>?", -1).Sum("uid")
	if err != nil {
		panic(err)
	}
	fmt.Printf("sum(uid): %v\n", n)

	// 求最大值
	n, err = db.Table("user").Where("uid>?", -1).Max("uid")
	if err != nil {
		panic(err)
	}
	fmt.Printf("max(uid): %v\n", n)

	// 求最小值
	n, err = db.Table("user").Where("uid>?", -1).Min("uid")
	if err != nil {
		panic(err)
	}
	fmt.Printf("min(uid): %v\n", n)

	// 自定义复杂 SQL 获取单条结果（原生）
	var uid int64
	err = db.QueryRow("SELECT uid FROM user WHERE uid=?", 2).Scan(&uid)
	if err != nil {
		panic(err)
	}
	fmt.Printf("uid: %v\n", uid)
	db.Table("user").LoadCache() // 自定义需要手动刷新缓存

	// 自定义复杂 SQL 获取多条（原生）
	var name string
	rows, err := db.Query("SELECT `uid`, `name` FROM `user` WHERE 1 ORDER BY uid DESC")
	if err != nil {
		panic(err)
	}
	rows.Close()
	for rows.Next() {
		rows.Scan(&uid, &name)
		fmt.Printf("uid: %v, name: %v\n", uid, name)
	}
	db.Table("user").LoadCache() // 自定义需要手动刷新缓存

	// 其他

	return
}
