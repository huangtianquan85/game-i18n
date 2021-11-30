package main

import (
	"database/sql"
	"fmt"
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
create table history (
	`id` bigint not null auto_increment primary key,
    `chinese` varchar(1024) not null,
    `english` varchar(1024) not null,
	`timestamp` bigint not null
) engine=innodb default charset=utf8;

create table translate (
    `chinese` varchar(1024) not null primary key,
	`timestamp` bigint not null,
    `source` varchar(512) not null,
    `useful` tinyint not null,
	`star` tinyint not null,
	`comment` varchar(1024) not null
) engine=innodb default charset=utf8;
*/

type Info struct {
	Chinese   string
	English   string
	Comment   string
	Star      int
	Timestamp string
}

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
