package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"time"
)

type Info struct {
	Translate string `json:"translate"`
	Source    string `json:"-"`
}

type Infos map[string]Info

func insertToEn(k *string, v *Info, tx *sql.Tx) error {
	stmt, err := tx.Prepare("INSERT INTO en (`keyHash`, `valueHash`, `value`, timestamp, userId) VALUES (?,?,?,?,?)")
	if err != nil {
		return fmt.Errorf("mysql prepare error %v", err)
	}

	_, err = stmt.Exec(StringMd5(*k), StringMd5(v.Translate), v.Translate, time.Now().UnixNano(), 0)
	if err != nil {
		return fmt.Errorf("mysql insert error at %s %v", *k, err)
	}

	return nil
}

func insertToMain(k *string, v *Info, tx *sql.Tx) error {
	stmt, err := tx.Prepare("INSERT INTO main (`keyHash`, `key`, `valueHash`, `source`, useful) VALUES (?,?,?,?,?)")
	if err != nil {
		return fmt.Errorf("mysql prepare error %v", err)
	}

	_, err = stmt.Exec(StringMd5(*k), *k, StringMd5(v.Translate), v.Source, 1)
	if err != nil {
		return fmt.Errorf("mysql insert error at %s %v", *k, err)
	}

	return nil
}

func LoadFromJson(path string) {
	// load json
	data, err := ioutil.ReadFile(path)
	if err != nil {
		fmt.Printf("read file error %v\n", err)
		return
	}

	// json unmarshal
	var m Infos
	err = json.Unmarshal(data, &m)

	if err != nil {
		fmt.Printf("json unmarshal error %v\n", err)
		return
	}

	// begin context
	tx, err := DB.Begin()
	if err != nil {
		fmt.Printf("mysql context begin error %v\n", err)
		return
	}

	// insert values
	for k, v := range m {
		err = insertToEn(&k, &v, tx)
		if err != nil {
			fmt.Printf("insert to en error %v\n", err)
			return
		}

		err = insertToMain(&k, &v, tx)
		if err != nil {
			fmt.Printf("insert to en error %v\n", err)
			return
		}
	}

	// mysql commit
	err = tx.Commit()
	if err != nil {
		fmt.Printf("mysql commit error %v\n", err)
		return
	}
}
