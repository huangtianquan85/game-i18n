package main

import (
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
)

const (
	user     = "root"
	password = "ksOy19ZWGMFV"
	host     = "mysql"
	port     = "3306"
	dbName   = "translate"
)

var DB *sql.DB

/*
create table en (
    `keyHash` char(32) not null,
	`valueHash` char(32) not null,
    `value` varchar(4096) not null,
	`timestamp` bigint not null,
	`userId` int not null,
	primary key (`keyHash`, `valueHash`)
) engine=innodb default charset=utf8mb4;

create table main (
    `keyhash` char(32) not null primary key,
	`key` varchar(4096) not null,
	`valueHash` char(32) not null,
    `source` varchar(512) not null,
    `useful` tinyint not null,
	`star` tinyint not null default 0,
	`comment` varchar(1024) not null default ''
) engine=innodb default charset=utf8mb4;
*/

func InitDB() error {
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
