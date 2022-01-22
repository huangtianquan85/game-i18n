package main

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/go-sql-driver/mysql"
)

var DB *sql.DB

/*
create table languages (
    `name` char(32) not null primary key
) engine=innodb default charset=utf8;

create table <history_lang> (
    `keyHash` char(32) not null,
	`valueHash` char(32) not null,
    `value` varchar(4096) not null,
	`timestamp` bigint not null,
	`userId` int not null,
	primary key (`keyHash`, `valueHash`)
) engine=innodb default charset=utf8;

create table <mapping_lang> (
    `keyHash` char(32) not null primary key,
	`valueHash` char(32) not null,
	`star` tinyint not null default 0,
	`comment` varchar(1024) not null default ''
) engine=innodb default charset=utf8;

create table key_info (
    `keyHash` char(32) not null primary key,
	`key` varchar(4096) not null
) engine=innodb default charset=utf8;

create table <branch_name> (
    `keyHash` char(32) not null primary key,
    `source` varchar(512) not null,
    `useful` tinyint not null
) engine=innodb default charset=utf8;
*/

func getEnv(name string, defaultValue string) string {
	v := os.Getenv(name)
	if v == "" {
		return defaultValue
	} else {
		return v
	}
}

func InitDB() error {
	user := getEnv("DB_USER", "root")
	password := getEnv("DB_PASSWD", "")
	host := getEnv("DB_HOST", "mysql")
	port := getEnv("DB_PORT", "3306")
	dbName := getEnv("DB_NAME", "translate")

	path := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", user, password, host, port, dbName)
	db, err := sql.Open("mysql", path)
	if err != nil {
		return fmt.Errorf("connect mysql error: %v", err)
	}

	db.SetConnMaxLifetime(100)
	db.SetMaxIdleConns(10)

	if err := db.Ping(); err != nil {
		return fmt.Errorf("ping mysql error: %v", err)
	}

	DB = db
	fmt.Println("connect mysql success")
	return nil
}
