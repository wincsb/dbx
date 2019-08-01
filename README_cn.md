# dbx:  一个支持 KV 缓存全表数据的高性能 golang db 库

> **dbx = MySQL/Sqlite3 + Memcached**

有 sqlx, gorm... 为什么要造这个轮子？

我的工作中需要处理大量日志类数据(多层嵌套的 json 格式，行数百亿级，大小 TB 级的数据），而且还有和远程的 DB 做业务数据关联，还要快速查询。
是的，非常棘手，我急切需要一个能够方便的可以支持全表缓存的 DB 库（主要用来缓存远程数据，本地 TB 级别的无法全部缓存），支持结构体嵌套，所以有了 dbx。
Redis, Memcached 也能初步满足需求，但是同时操作 cache, db 会比较麻烦，会有一致性的问题，代码也啰嗦很多。
所以就诞生了 dbx，目前支持 MySQL/Sqlite3，透明支持按照行缓存数据，支持结构体自由组合嵌套，它仅仅依赖以下类库：
```bash
go get "github.com/go-sql-driver/mysql"
go get "github.com/mattn/go-sqlite3"
```

# 支持缓存，高性能读取 KV 缓存全表数据
经过本机简单的测试（小数据下），直接查询 Sqlite3 速度可以达到 3w+/s，开启缓存后达到恐怖的 350w+/s，足够用了。
针对高频访问的小表开启缓存：
```golang
db.BindStruct("user", &User{}, true)
db.BindStruct("group", &Group{}, true)
db.EnableCache(true)
```

# 支持嵌套，尽量避免低效反射
golang 为静态语言，在实现比较复杂的功能的时候往往要用到反射，而反射使用不当的时候会严重拖慢速度。经过实践发现，应该尽量使用数字索引，不要使用字符串索引，比如 Field() 性能大约是 FieldByName() 的 50 倍！
绝大部分 db 库不支持嵌套，因为反射又慢又复杂，特别是嵌套层数过多的时候。还好通过努力，dbx 高效的实现了无限制层数的嵌套支持，并且性能还不错。
```golang
type Human struct {
	Age int64     `db:"age"`
}
type User struct {
	Human
	Uid        int64     `db:"uid"`
	Gid        int64     `db:"gid"`
	Name       string    `db:"name"`
	CreateDate time.Time `db:"createDate"`
}
```

# 优美的 API
通过 golang 的反射特性，可以实现接近脚本语言级的便捷程度。如下：
```golang

// 打开数据库
db, err = dbx.Open("mysql", "root@tcp(localhost)/test?parseTime=true&charset=utf8")

// insert one / 插入一条
db.Table("user").Insert(u1)

// find one / 查询一条
db.Table("user").WherePK(1).One(u2)

// update one / 更新一条
db.Table("user").Update(u2)

// delete one /  删除一条
db.Table("user").WherePK(1).Delete()

// find multi / 获取多条
db.Table("user").Where("uid>?", 1).All(&userList)

```

# 日志输出到指定的流
这种设计应该推广到所有的复杂类库。
```golang
// 将 db 产生的错误信息输出到标准输出（控制台）
db.Stderr = os.Stdout

// 将 db 产生的错误信息输出到指定的文件
db.Stderr = dbx.OpenLogFile("./db_error.log") 

// 默认：将 db 的输出（主要为 SQL 语句）重定向到"黑洞"（不输出执行的 SQL 语句等信息）
db.Stdout = ioutil.Discard

// 默认：将 db 产生的输出（主要为 SQL 语句）输出到标准输出（控制台）
db.Stdout = os.Stdout
```

# 兼容原生的方法
有时候我们需要调用原生的接口，来实现比较复杂的目的。
```golang
// 自定义复杂 SQL 获取单条结果（原生）
var uid int64
err = db.QueryRow("SELECT uid FROM user WHERE uid=?", 2).Scan(&uid)
if err != nil {
	panic(err)
}
fmt.Printf("uid: %v\n", uid)
db.Table("user").FlushCache() // 自定义需要手动刷新缓存
```

# 用例
如此简单，以至于只需要贴一段测试代码，就可以掌握其用法：
```golang
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

	// db, err = dbx.Open("sqlite3", "./db1.db?cache=shared&mode=rwc&parseTime=true&charset=utf8") // sqlite3
	db, err = dbx.Open("mysql", "root@tcp(localhost)/test?parseTime=true&charset=utf8")            // mysql
	if err != nil {
		panic(err)
	}
	defer db.Close()

	// db 输出信息设置
	db.Stdout = os.Stdout // 将 db 产生的信息(大部分为 sql 语句)输出到标准输出
	db.Stderr = dbx.OpenLogFile("./db_error.log") // 将 db 产生的错误信息输出到指定的文件
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
	db.BindStruct("user2", &User{}, true)
	db.EnableCache(true)

	// 插入一条
	u1 := &User{1, 1, "jack", time.Now()}
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
	u2.Name = "jack.ma"
	_, err = db.Table("user").Update(u2)
	if err != nil {
		panic(err)
	}

	// 删除一条
	_, err = db.Table("user").WherePK(1).Delete()
	if err != nil {
		panic(err)
	}

	// Where 条件 + 更新
	_, err = db.Table("user").WhereM(dbx.M{{"uid", 1}, {"gid", 1}}).UpdateM(dbx.M{{"Name", "jet.li"}})
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

	return
}

```
[Document for English](README.md)
