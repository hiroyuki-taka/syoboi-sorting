package main

import (
	"fmt"
	"github.com/goccy/go-json"
	"github.com/spiegel-im-spiegel/errs"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
)

type TitleMediumResponse struct {
	Titles map[string]TitleMedium `json:"Titles"`
}

type TitleMedium struct {
	TID   string `json:"TID"`
	Title string `json:"Title"`
}

type Config struct {
	RootDir string `json:"rootDir"`
}

var userAgent0 = fmt.Sprintf("syoboi-sorting/%s (%s)", "1.0.0", runtime.Version())

func main() {
	// config読み込み
	config, err := LoadConfig()
	if err != nil {
		print(fmt.Sprintf("%-v", err))
		return
	}

	// 番組情報取得
	programs, err := GetAllPrograms()
	if err != nil {
		print(fmt.Sprintf("%-v", err))
		return
	}

	// チャネル準備
	jobChan := make(chan TitleMedium, 4)
	done := make(chan struct{})
	go worker(config, jobChan, done)

	// 番組毎に分けてチャネルへ投げる
	for _, p := range programs {
		jobChan <- p
	}
	done <- struct{}{}
}

func worker(config *Config, jobChan chan TitleMedium, done chan struct{}) {
	rootDir := config.RootDir
	files, err := ioutil.ReadDir(rootDir)
	if err != nil {
		return
	}

	for {
		select {
		case tm := <-jobChan:
			// - 番組名を '-[番組名] #' に変形
			searchTitle := `-(\[.\])*\Q` + tm.Title + `\E #`
			r, err := regexp.Compile(searchTitle)
			if err != nil {
				fmt.Println(err)
				continue
			}

			// 移動先ディレクトリ名=[TID] [番組名]
			moveToDir := path.Join(rootDir, fmt.Sprintf("%s %s", tm.TID, tm.Title))

			// - 部分一致でファイル検索
			for _, file := range files {
				if r.MatchString(file.Name()) {
					// - 移動先ディレクトリがなければ作成
					_, err := os.Stat(moveToDir)
					if err != nil {
						_ = os.Mkdir(moveToDir, 0777)
					}

					// - 見つかったファイルを移動先に移動
					moveFromFile := filepath.Join(rootDir, file.Name())
					moveToFile := filepath.Join(moveToDir, file.Name())

					fmt.Printf(" move %s -> %s\n", moveFromFile, moveToFile)
					_ = os.Rename(moveFromFile, moveToFile)
				}
			}
		//
		case <-done:
			return
		}
	}
}

func LoadConfig() (*Config, error) {
	fs, err := os.Open("config.json")
	if err != nil {
		return nil, errs.New("configファイルの読み込みに失敗しました", errs.WithCause(err))
	}

	defer fs.Close()

	config := new(Config)
	err = json.NewDecoder(fs).Decode(&config)
	if err != nil {
		return nil, errs.New("configファイルの読み込みに失敗しました", errs.WithCause(err))
	}

	return config, nil
}

func GetAllPrograms() (map[string]TitleMedium, error) {
	req, _ := http.NewRequest(http.MethodGet, "https://cal.syoboi.jp/json.php?Req=TitleMedium", nil)
	req.Header.Set("User-Agent", userAgent0)

	client := new(http.Client)
	resp, err := client.Do(req)
	if err != nil {
		return nil, errs.New("TitleMedium呼び出しエラー", errs.WithCause(err))
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, errs.New("TitleMediumステータスコード不正", errs.WithContext("StatusCode", resp.StatusCode))
	}
	data := new(TitleMediumResponse)

	err = json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		return nil, errs.New("TitleMedium jsonパースエラー", errs.WithCause(err))
	}

	return data.Titles, nil
}
