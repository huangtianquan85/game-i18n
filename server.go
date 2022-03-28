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

// 获取用于打包的翻译信息
func translates(r *http.Request) ([]byte, error) {
	// 判断分支
	values := r.URL.Query()
	branch := values.Get("branch")
	if branch == "" {
		return nil, fmt.Errorf("branch can not empty")
	}

	// 获取语言列表
	langs := make([]string, 0)
	langInfos := make(map[string]*pb.LanguageInfo)
	rows, err := DB.Query("SELECT * from languages ORDER BY `showOrder`;")
	if err != nil {
		return nil, fmt.Errorf("query error: %v", err)
	}

	for rows.Next() {
		var tableName string
		var showName string
		var unityEnum string
		var showIndex int
		rows.Scan(&tableName, &showName, &unityEnum, &showIndex)
		info := &pb.LanguageInfo{}
		if tableName == "zh-cn" {
			info.Index = -1
		} else {
			info.Index = int32(len(langs))
			langs = append(langs, tableName)
		}
		info.ShowName = showName
		info.UnityEnums = strings.Split(unityEnum, ",")
		langInfos[tableName] = info
	}

	rows.Close()

	root := pb.Root{
		Table: make(map[string]*pb.Languages),
		Langs: langInfos,
	}

	// 生成各语言查询语句
	for _, l := range langs {
		queryCmd := fmt.Sprintf(`
		SELECT key_info.key, history_<lang>.value
		FROM
		(SELECT keyhash FROM branch_%s WHERE useful = 1)
		as selection
		LEFT JOIN key_info ON selection.keyhash = key_info.keyhash
		LEFT JOIN mapping_<lang> ON selection.keyhash = mapping_<lang>.keyhash
		LEFT JOIN history_<lang> ON mapping_<lang>.keyhash = history_<lang>.keyHash and mapping_<lang>.valueHash = history_<lang>.valueHash
		ORDER BY key_info.key`, branch)
		queryCmd = strings.ReplaceAll(queryCmd, "<lang>", l)

		// query all useful rows
		rows, err = DB.Query(queryCmd)
		if err != nil {
			return nil, fmt.Errorf("query error: %v", err)
		}

		var key string
		var value string
		l := len(langs)
		for rows.Next() {
			rows.Scan(&key, &value)

			if _, ok := root.Table[key]; !ok {
				root.Table[key] = &pb.Languages{
					Translate: make([]string, 0, l),
				}
			}
			root.Table[key].Translate = append(root.Table[key].Translate, value)
		}

		rows.Close()
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
	English   string
	Timestamp string
	UserId    int
	Source    string
	Star      int
	Comment   string
	Useful    bool
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
	SELECT <branch>.keyHash, <branch>.source, <branch>.useful, key_info.key, mapping_<lang>.valueHash, mapping_<lang>.star, mapping_<lang>.comment, history_<lang>.value, history_<lang>.timestamp, history_<lang>.userId
	FROM <branch>
	LEFT JOIN key_info ON <branch>.keyHash = key_info.keyHash
	LEFT JOIN mapping_<lang> ON <branch>.keyHash = mapping_<lang>.keyHash
	LEFT JOIN history_<lang> ON mapping_<lang>.keyHash = history_<lang>.keyHash and mapping_<lang>.valueHash = history_<lang>.valueHash
	`
	queryCmd = strings.ReplaceAll(queryCmd, "<branch>", "branch_"+branch)
	queryCmd = strings.ReplaceAll(queryCmd, "<lang>", lang)

	rows, err := DB.Query(queryCmd)
	if err != nil {
		return nil, fmt.Errorf("query error: %v", err)
	}

	// scan to editor info
	infos := make(map[string]*editorInfo, 0)
	for rows.Next() {
		i := new(editorInfo)
		var t int64
		rows.Scan(&i.KeyHash,
			&i.Source,
			&i.Useful,
			&i.Key,
			&i.ValueHash,
			&i.Star,
			&i.Comment,
			&i.Value,
			&t,
			&i.UserId)
		i.Timestamp = strconv.FormatInt(t, 10)
		infos[i.KeyHash] = i
	}

	rows.Close()

	if lang != "en" {
		queryCmd = `
		SELECT <branch>.keyHash, history_<lang>.value
		FROM <branch>
		LEFT JOIN mapping_<lang> ON <branch>.keyHash = mapping_<lang>.keyHash
		LEFT JOIN history_<lang> ON mapping_<lang>.keyHash = history_<lang>.keyHash and mapping_<lang>.valueHash = history_<lang>.valueHash
		`
		queryCmd = strings.ReplaceAll(queryCmd, "<branch>", "branch_"+branch)
		queryCmd = strings.ReplaceAll(queryCmd, "<lang>", "en")

		rows, err := DB.Query(queryCmd)
		if err != nil {
			return nil, fmt.Errorf("query error: %v", err)
		}

		defer rows.Close()

		var keyHash string
		var english string
		for rows.Next() {
			rows.Scan(&keyHash, &english)
			i, ok := infos[keyHash]
			if ok {
				i.English = english
			}
		}
	}

	// convert map to array
	arr := make([]*editorInfo, 0, len(infos))
	for _, v := range infos {
		arr = append(arr, v)
	}

	// convert to json
	data, err := json.Marshal(arr)
	if err != nil {
		return nil, fmt.Errorf("marshal error: %v", err)
	}

	return data, nil
}

// 获取语言列表
func languages(r *http.Request) ([]byte, error) {
	langs := make([]string, 0)
	rows, err := DB.Query("SELECT * from languages ORDER BY `showOrder`;")
	if err != nil {
		return nil, fmt.Errorf("query error: %v", err)
	}

	defer rows.Close()

	for rows.Next() {
		var tableName string
		var showName string
		var unityEnum string
		var showIndex int
		rows.Scan(&tableName, &showName, &unityEnum, &showIndex)
		if tableName != "zh-cn" {
			langs = append(langs, tableName)
		}
	}

	// convert to json
	data, err := json.Marshal(langs)
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
	values := r.URL.Query()
	lang := values.Get("lang")
	if lang == "" {
		return fmt.Errorf("lang can not empty")
	}

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
		// insert to history
		cmd := fmt.Sprintf("insert ignore into `history_%s` (`keyHash`, `valueHash`, `value`, timestamp, userId) values (?,?,?,?,?);", lang)
		_, err = tx.Exec(cmd, StringMd5(t.Key), StringMd5(t.Value), t.Value, time.Now().UnixNano(), 0)
		if err != nil {
			return fmt.Errorf("insert error: %v", err)
		}

		// update mapping
		cmd = fmt.Sprintf(`
		INSERT INTO mapping_%s (keyHash, valueHash, star, comment)
		VALUES (?,?,?,?)
		ON DUPLICATE KEY
		UPDATE valueHash=VALUES(valueHash), star=VALUES(star), comment=VALUES(comment)`, lang)
		_, err = tx.Exec(cmd, StringMd5(t.Key), StringMd5(t.Value), t.Star, t.Comment)
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
	values := r.URL.Query()
	branch := values.Get("branch")
	if branch == "" {
		return fmt.Errorf("branch can not empty")
	}

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
	queryCmd := strings.ReplaceAll(`
	SELECT key_info.key, branch_<branch>.useful 
	FROM branch_<branch> 
	LEFT JOIN key_info 
	ON key_info.keyHash = branch_<branch>.keyHash;
	`, "<branch>", branch)
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
	cmd := strings.ReplaceAll("UPDATE branch_<branch> SET useful=? WHERE keyHash=?", "<branch>", branch)
	for rows.Next() {
		var key string
		var useful int
		rows.Scan(&key, &useful)
		dbItems[key] = useful == 1

		_, ok := m[key]
		if (useful == 1) != ok { // 状态不一样
			useful = 1 - useful
			tx.Exec(cmd, useful, StringMd5(key))
			if err != nil {
				return fmt.Errorf("update useful error: %v", err)
			}
		}
	}

	// insert to branch_<branch>
	cmd = strings.ReplaceAll("INSERT ignore INTO branch_<branch> (`keyHash`, `source`, useful) VALUES (?,?,?);", "<branch>", branch)
	for k := range m {
		keyHash := StringMd5(k)
		if _, ok := dbItems[k]; !ok {
			_, err = tx.Exec(cmd, keyHash, "", 1)
			if err != nil {
				return fmt.Errorf("insert new item to branch_%s error: %v", branch, err)
			}
			_, err = tx.Exec("INSERT ignore INTO key_info (`keyHash`, `key`) VALUES (?,?);", keyHash, k)
			if err != nil {
				return fmt.Errorf("insert new item to key_info error: %v", err)
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
	addHttpBodyHandler("languages", languages)
	addHttpSimpleHandler("update-sources", updateSources)
	addHttpBodyHandler("auto-translate", autoTranslate)
	addHttpBodyHandler("get-screen", getScreen)
	addHttpSimpleHandler("set-screen", setScreen)
	http.HandleFunc("/", index)
	http.ListenAndServe("0.0.0.0:8081", nil)
}
