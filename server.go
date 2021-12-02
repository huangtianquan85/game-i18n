package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"
	"translate/pb"

	"google.golang.org/protobuf/proto"
)

// 获取用于打包的翻译信息
func translates(w http.ResponseWriter, r *http.Request) {
	queryCmd := `
	SELECT m.key, en.value FROM 
		(SELECT * FROM main WHERE main.useful = 1) as m 
		LEFT JOIN en 
		ON m.keyhash = en.keyHash and m.valueHash = en.valueHash;
	`
	// query all useful rows
	rows, err := DB.Query(queryCmd)
	if err != nil {
		fmt.Fprintln(w, "query error", err)
		w.WriteHeader(500)
		return
	}

	// scan to editor info
	root := pb.Root{
		Table: make(map[string]*pb.Languages),
	}
	var key string
	var value string
	for rows.Next() {
		rows.Scan(&key, &value)
		root.Table[key] = &pb.Languages{
			Translate: []string{value},
		}
	}

	// convert to pb
	data, err := proto.Marshal(&root)
	if err != nil {
		fmt.Fprintln(w, "marshal error", err)
		w.WriteHeader(500)
		return
	}

	w.Write(data)
}

type editorInfo struct {
	KeyHash   string
	Key       string
	ValueHash string
	Value     string
	Timestamp string
	UserId    int
	Source    string
	Star      int
	Comment   string
}

// 获取用于编辑器的翻译信息
func translatesEditor(w http.ResponseWriter, r *http.Request) {
	queryCmd := `
	SELECT m.keyHash, m.key, m.valuehash, m.source, m.star, m.comment, en.value, en.timestamp, en.userId FROM 
		(SELECT * FROM main WHERE main.useful = 1) as m 
		LEFT JOIN en 
		ON m.keyhash = en.keyHash and m.valueHash = en.valueHash;
	`
	// query all useful rows
	rows, err := DB.Query(queryCmd)
	if err != nil {
		fmt.Fprintln(w, "query error", err)
		w.WriteHeader(500)
		return
	}

	// scan to editor info
	infos := make([]editorInfo, 0)
	for rows.Next() {
		i := new(editorInfo)
		var t int64
		rows.Scan(&i.KeyHash,
			&i.Key,
			&i.ValueHash,
			&i.Source,
			&i.Star,
			&i.Comment,
			&i.Value,
			&t,
			&i.UserId)
		i.Timestamp = strconv.FormatInt(t, 10)
		infos = append(infos, *i)
	}

	// convert to json
	data, err := json.Marshal(infos)
	if err != nil {
		fmt.Fprintln(w, "marshal error", err)
		w.WriteHeader(500)
		return
	}

	w.Header().Set("content-type", "text/json")
	w.Write(data)
}

// 获取一个翻译的所有历史
// 更新评分，注释（注释属于历史还是属于翻译呢？）

type commitInfo struct {
	Key     string
	Value   string
	Star    int
	Comment string
}

func commit(r *http.Request) error {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return fmt.Errorf("commit-translate read body error: %v", err)
	}

	// unmarshal
	var t commitInfo
	err = json.Unmarshal(body, &t)
	if err != nil {
		return fmt.Errorf("commit-translate unmarshal error: %v", err)
	}

	// begin context
	tx, err := DB.Begin()
	if err != nil {
		return fmt.Errorf("context begin error: %v", err)
	}

	// insert to en
	cmd := "insert ignore into `en` (`keyHash`, `valueHash`, `value`, timestamp, userId) values (?,?,?,?,?);"
	_, err = tx.Exec(cmd, StringMd5(t.Key), StringMd5(t.Value), t.Value, time.Now().UnixNano(), 0)
	if err != nil {
		return fmt.Errorf("insert error: %v", err)
	}

	// update main
	cmd = "UPDATE main SET main.valueHash=?, main.star=?, main.comment=? WHERE main.keyHash=?"
	_, err = tx.Exec(cmd, StringMd5(t.Value), t.Star, t.Comment, StringMd5(t.Key))
	if err != nil {
		return fmt.Errorf("update error: %v", err)
	}

	// commit
	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("commit error: %v", err)
	}

	return nil
}

// 提交翻译
func commitTranslate(w http.ResponseWriter, r *http.Request) {
	if err := commit(r); err != nil {
		fmt.Println(err)
		w.WriteHeader(500)
		w.Write([]byte(err.Error()))
	}
}

func update(r *http.Request) error {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return fmt.Errorf("update-sources read body error: %v", err)
	}

	// unmarshal
	var f interface{}
	err = json.Unmarshal(body, &f)
	if err != nil {
		return fmt.Errorf("update-sources unmarshal error: %v", err)
	}

	m := f.(map[string]interface{})

	// query all rows
	queryCmd := "SELECT `main`.`key`, `main`.`useful` from main;"
	rows, err := DB.Query(queryCmd)
	if err != nil {
		return fmt.Errorf("query error: %v", err)
	}

	// begin context
	tx, err := DB.Begin()
	if err != nil {
		return fmt.Errorf("context begin error: %v", err)
	}

	// find useless items
	dbItems := make(map[string]bool)
	for rows.Next() {
		var key string
		var useful int
		rows.Scan(&key, &useful)
		dbItems[key] = useful == 1

		_, ok := m[key]
		if (useful == 1) != ok { // 状态不一样
			useful = 1 - useful
			tx.Exec("UPDATE main SET main.useful=? WHERE main.keyHash=?", useful, StringMd5(key))
			if err != nil {
				return fmt.Errorf("update useful error: %v", err)
			}
		}
	}

	// insert to main
	cmd := "INSERT ignore INTO main (`keyHash`, `key`, `valueHash`, `source`, useful) VALUES (?,?,?,?,?);"
	for k := range m {
		if _, ok := dbItems[k]; !ok {
			_, err = tx.Exec(cmd, StringMd5(k), k, StringMd5(""), "", 1)
			if err != nil {
				return fmt.Errorf("insert new item error: %v", err)
			}
		}
	}

	// commit
	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("commit error: %v", err)
	}

	return nil
}

// 更新翻译需求
func updateSources(w http.ResponseWriter, r *http.Request) {
	if err := update(r); err != nil {
		fmt.Println(err)
		w.WriteHeader(500)
		w.Write([]byte(err.Error()))
	}
}

func handleAutoTranslate(r *http.Request) ([]byte, error) {
	url, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("auto-translate read body error: %v", err)
	}

	resp, err := http.Get(string(url))
	if err != nil {
		return nil, fmt.Errorf("auto-translate request error: %v", err)
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("auto-translate request error: %v", err)
	}

	return body, nil
}

// 代理自动翻译，解决跨域问题
func autoTranslate(w http.ResponseWriter, r *http.Request) {
	body, err := handleAutoTranslate(r)
	if err != nil {
		fmt.Println(err)
		w.WriteHeader(500)
		w.Write([]byte(err.Error()))
	}

	if body != nil {
		w.Write(body)
	}
}

func index(w http.ResponseWriter, r *http.Request) {
	body, _ := ioutil.ReadFile("index.html")
	fmt.Fprint(w, string(body))
}

func StartServer() {
	http.HandleFunc("/commit-translate", commitTranslate)
	http.HandleFunc("/translates", translates)
	http.HandleFunc("/translates-editor", translatesEditor)
	http.HandleFunc("/update-sources", updateSources)
	http.HandleFunc("/auto-translate", autoTranslate)
	http.HandleFunc("/", index)
	http.ListenAndServe("0.0.0.0:8081", nil)
}
