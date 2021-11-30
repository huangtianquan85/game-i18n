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
	Key   string
	Value string
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
	cmd = "UPDATE main SET main.valueHash=? WHERE main.keyHash=?"
	_, err = tx.Exec(cmd, StringMd5(t.Value), StringMd5(t.Key))
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

// 更新翻译需求
func newSources(w http.ResponseWriter, r *http.Request) {
	// rows, err := DB.Query(string(body))
	// if err != nil {
	// 	fmt.Fprintln(w, "query error", err)
	// 	w.WriteHeader(500)
	// 	return
	// }

	// defer rows.Close()

	// infos := make([]Info, 0)
	// for rows.Next() {
	// 	i := new(Info)
	// 	rows.Scan(&i.Origin, &i.Translate, &i.StackHash, &i.Stack, &i.Comment, &i.ScreenshotHash, &i.Version)
	// 	infos = append(infos, *i)
	// }

	// data, err := json.Marshal(infos)
	// if err != nil {
	// 	fmt.Fprintln(w, "query error", err)
	// 	w.WriteHeader(500)
	// 	return
	// }
}

func index(w http.ResponseWriter, r *http.Request) {
	body, _ := ioutil.ReadFile("index.html")
	fmt.Fprint(w, string(body))
}

func StartServer() {
	http.HandleFunc("/commit-translate", commitTranslate)
	http.HandleFunc("/translates", translates)
	http.HandleFunc("/translates-editor", translatesEditor)
	http.HandleFunc("/new-sources", newSources)
	http.HandleFunc("/", index)
	http.ListenAndServe("0.0.0.0:8081", nil)
}
