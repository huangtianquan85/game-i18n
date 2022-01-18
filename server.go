package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
	"translate/pb"

	"google.golang.org/protobuf/proto"
)

var langs = []string{
	"en",
	"ge",
}

// 获取用于打包的翻译信息
func translates(r *http.Request) ([]byte, error) {
	// 判断分支
	values := r.URL.Query()
	branch := values.Get("branch")
	if branch == "" {
		return nil, fmt.Errorf("branch can not empty")
	}

	// 生成各语言字段
	fields := make([]string, 0)
	for _, l := range langs {
		fields = append(fields, fmt.Sprintf("history_%s.value", l))
	}

	// 按照分支筛选
	queryCmd := fmt.Sprintf(`
	SELECT key_info.key, %s 
	FROM
	(SELECT keyhash FROM branch_%s WHERE useful = 1) 
	as selection
	LEFT JOIN key_info ON selection.keyhash = key_info.keyhash`,
		strings.Join(fields, ", "), branch)

	// 生成各语言 Join 语句
	for _, l := range langs {
		queryCmd += strings.ReplaceAll(`
	LEFT JOIN mapping_<lang> ON selection.keyhash = mapping_<lang>.keyhash
	LEFT JOIN history_<lang> ON mapping_<lang>.keyhash = history_<lang>.keyHash and mapping_<lang>.valueHash = history_<lang>.valueHash`,
			"<lang>", l)
	}

	// query all useful rows
	rows, err := DB.Query(queryCmd)
	if err != nil {
		return nil, fmt.Errorf("query error: %v", err)
	}

	defer rows.Close()

	// scan to pb
	root := pb.Root{
		Table: make(map[string]*pb.Languages),
		Langs: make(map[string]uint32),
	}

	var key string
	for rows.Next() {
		values := make([]string, len(langs))
		points := make([]interface{}, len(langs)+1)
		points[0] = &key

		for i := range langs {
			points[i+1] = &values[i]
		}

		rows.Scan(points...)

		root.Table[key] = &pb.Languages{
			Translate: values,
		}
	}

	for i, l := range langs {
		root.Langs[l] = uint32(i)
	}

	// convert to pb
	data, err := proto.Marshal(&root)
	if err != nil {
		return nil, fmt.Errorf("marshal error: %v", err)
	}

	return data, nil
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
func translatesEditor(r *http.Request) ([]byte, error) {
	values := r.URL.Query()
	branch := values.Get("branch")
	lang := values.Get("lang")

	if branch == "" || lang == "" {
		return nil, fmt.Errorf("branch and lang can not empty")
	}

	queryCmd := `
	SELECT selection.keyHash, key_info.key, <mapping>.valueHash, selection.source, <mapping>.star, <mapping>.comment, <history>.value, <history>.timestamp, <history>.userId
	FROM
	(SELECT * FROM <branch> WHERE useful = 1) 
	as selection
	LEFT JOIN key_info ON selection.keyHash = key_info.keyHash
	LEFT JOIN <mapping> ON selection.keyHash = <mapping>.keyHash
	LEFT JOIN <history> ON <mapping>.keyHash = <history>.keyHash and <mapping>.valueHash = <history>.valueHash
	`
	queryCmd = strings.ReplaceAll(queryCmd, "<branch>", "branch_"+branch)
	queryCmd = strings.ReplaceAll(queryCmd, "<mapping>", "mapping_"+lang)
	queryCmd = strings.ReplaceAll(queryCmd, "<history>", "history_"+lang)

	// query all useful rows
	rows, err := DB.Query(queryCmd)
	if err != nil {
		return nil, fmt.Errorf("query error: %v", err)
	}

	defer rows.Close()

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
		return nil, fmt.Errorf("marshal error: %v", err)
	}

	return data, nil
}

// 获取一个翻译的所有历史
// 更新评分，注释（注释属于历史还是属于翻译呢？）

type commitInfo struct {
	Key     string
	Value   string
	Star    int
	Comment string
}

// 提交翻译
func commitTranslate(r *http.Request) error {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return fmt.Errorf("commit-translate read body error: %v", err)
	}

	// unmarshal
	items := make([]commitInfo, 0)
	err = json.Unmarshal(body, &items)
	if err != nil {
		return fmt.Errorf("commit-translate unmarshal error: %v", err)
	}

	// begin context
	tx, err := DB.Begin()
	if err != nil {
		return fmt.Errorf("context begin error: %v", err)
	}

	for _, t := range items {
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
	}

	// commit
	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("commit error: %v", err)
	}

	return nil
}

// 更新翻译需求
func updateSources(r *http.Request) error {
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

	defer rows.Close()

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

// 代理自动翻译，解决跨域问题
func autoTranslate(r *http.Request) ([]byte, error) {
	url, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("read body error: %v", err)
	}

	resp, err := http.Get(string(url))
	if err != nil {
		return nil, fmt.Errorf("request upstream error: %v", err)
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read upstream error: %v", err)
	}

	return body, nil
}

var screenTexts = make(map[string][]string)
var mutex sync.Mutex

// 刷新界面显示文字
func setScreen(r *http.Request) error {
	values := r.URL.Query()
	token := values.Get("token")
	if token == "" {
		return fmt.Errorf("token can not empty")
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return fmt.Errorf("read body error: %v", err)
	}

	// unmarshal
	var keys []string
	err = json.Unmarshal(body, &keys)
	if err != nil {
		return fmt.Errorf("unmarshal error: %v", err)
	}

	mutex.Lock()
	defer mutex.Unlock()

	screenTexts[token] = keys

	return nil
}

// 接收界面显示文字
func getScreen(r *http.Request) ([]byte, error) {
	values := r.URL.Query()
	token := values.Get("token")
	if token == "" {
		return nil, fmt.Errorf("token can not empty")
	}

	mutex.Lock()
	defer mutex.Unlock()

	if texts, ok := screenTexts[token]; ok {
		body, err := json.Marshal(texts)
		if err != nil {
			return nil, fmt.Errorf("marshal error: %v", err)
		}
		return body, nil
	}

	return nil, nil
}

func addHttpSimpleHandler(tag string, handler func(*http.Request) error) {
	f := func(w http.ResponseWriter, r *http.Request) {
		if err := handler(r); err != nil {
			fmt.Println(err)
			w.WriteHeader(500)
			w.Write([]byte(tag + " => " + err.Error()))
		}
	}

	http.HandleFunc("/"+tag, f)
}

func addHttpBodyHandler(tag string, handler func(*http.Request) ([]byte, error)) {
	f := func(w http.ResponseWriter, r *http.Request) {
		body, err := handler(r)
		if err != nil {
			fmt.Println(err)
			w.WriteHeader(500)
			w.Write([]byte(tag + " => " + err.Error()))
		}

		if body != nil {
			w.Write(body)
		}
	}

	http.HandleFunc("/"+tag, f)
}

func index(w http.ResponseWriter, r *http.Request) {
	body, _ := ioutil.ReadFile("index.html")
	fmt.Fprint(w, string(body))
}

func StartServer() {
	addHttpSimpleHandler("commit-translate", commitTranslate)
	addHttpBodyHandler("translates", translates)
	addHttpBodyHandler("translates-editor", translatesEditor)
	addHttpSimpleHandler("update-sources", updateSources)
	addHttpBodyHandler("auto-translate", autoTranslate)
	addHttpBodyHandler("get-screen", getScreen)
	addHttpSimpleHandler("set-screen", setScreen)
	http.HandleFunc("/", index)
	http.ListenAndServe("0.0.0.0:8081", nil)
}
