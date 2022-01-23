package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/go-co-op/gocron"
	"github.com/go-redis/redis"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type TimestampRange []int64

func (a TimestampRange) Len() int           { return len(a) }
func (a TimestampRange) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a TimestampRange) Less(i, j int) bool { return a[i] < a[j] }


type Reading struct {
	Time string
	Reading float32
}

const timeFormat = "15:04:05"

func msToTime(ms string) (string, error) {
	msInt, err := strconv.ParseInt(ms, 10, 64)
	if err != nil {
		return time.Time{}.String(), err
	}

	return time.Unix(msInt, 0).Format(timeFormat), nil
}

func msToTimeNonFormatted(ms string) (time.Time, error) {
	msInt, err := strconv.ParseInt(ms, 10, 64)
	if err != nil {
		return time.Time{}, err
	}

	return time.Unix(msInt, 0), nil
}

func convertTemplate(timestamp string, dataPath string) {
	content, err := ioutil.ReadFile("static/data_render_template.html")
	if err != nil {
		log.Fatal(err)
	}

	// Convert []byte to string and print to screen
	input := string(content)
	fmt.Println(input)

	fmt.Println("html file path:", dataPath + "/index_"+timestamp + ".html")

	data := make(map[string]interface{}, 4)
	data["timestamp"] = timestamp

	// Use os.Create to create a file for writing.
	f, _ := os.Create(dataPath + "/index_"+timestamp + ".html")

	// Create a new writer.
	w := bufio.NewWriter(f)

	// Write a string to the file.
	w.WriteString(AddSimpleTemplate(input,data))

	// Flush.
	w.Flush()
	fmt.Println(AddSimpleTemplate(input,data))
}

func generateFileList(dataPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		filesWithTimestamp := make(map[int64]string)

		err := filepath.Walk(dataPath, func(path string, info os.FileInfo, err error) error {
			if strings.Contains(path, "index_") {
				filesWithTimestamp[info.ModTime().Unix()] = path
			}
			return nil
		})
		log.Println("files from folder:", filesWithTimestamp)
		var keys []int64
		for k := range filesWithTimestamp {
			keys = append(keys, k)
		}

		sort.Sort(TimestampRange(keys))
		log.Println(keys)
		var files[]string
		for  _, key := range keys {
			filePaths := strings.Split(filesWithTimestamp[key], "/")
			files = append(files, filePaths[len(filePaths)-1])
		}

		if err != nil {
			panic(err)
		}

		tpl := template.Must(template.ParseGlob("static/filelist_template.html"))
		err1 := tpl.Execute(w, files)
				if err1 != nil {
					log.Fatalln(err1)
				}

	}

}

func AddSimpleTemplate(a string,b map[string]interface{}) string {
	tmpl := template.Must(template.New("email.tmpl").Parse(a))
	buf := &bytes.Buffer{}
	err := tmpl.Execute(buf, b)
	if err != nil {
		panic(err)
	}
	s := buf.String()
	return s
}

func Exists(name string) bool {
	if _, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

func generateHTML(client *redis.Client, dataPath string) {
	log.Println("Calling redis client to get the data to generate html")
	zRevRange := client.ZRevRange("meter-rank", 0, 1)
	var indexRanges []int64
	var timeStampAtEndIndex string
	for i, s := range zRevRange.Val() {
		log.Println("index range values from redis: ", i, s)
		var readingKeyValue = strings.Split(s, ":")
		msInt, _ := strconv.ParseInt(readingKeyValue[1], 10, 64)
		log.Println("index range:", msInt )
		indexRanges = append(indexRanges,msInt)
		if i == 0 {
			timeStampAtEndIndex = readingKeyValue[0]
		}
	}
	startIndex := indexRanges[1]
	endIndex := indexRanges[0]
	fileName := dataPath+ "/data_"+timeStampAtEndIndex+".json"
	if !Exists(fileName) {
		zRevRange1 := client.ZRange("meter-reading", startIndex, endIndex)
		var readings[] Reading

		log.Println("Start Index, end Index: ", startIndex, endIndex)
		for i, s := range zRevRange1.Val() {
			log.Println("Values in range generating html:", i, s)
			var readingKeyValue = strings.Split(s, ":")
			var value, _ = strconv.ParseFloat(readingKeyValue[1], 32)
			log.Println("reading value generating html:", value)
			var timeFormatted, _ = msToTime(readingKeyValue[0])
			readings = append(readings, Reading{timeFormatted, float32(value)})
		}
		log.Println("readings", readings)
		data, _ := json.MarshalIndent(readings, "", "")
		log.Println("writing json file: ", fileName)
		_ = ioutil.WriteFile(fileName, data, 0644)
		convertTemplate(timeStampAtEndIndex, dataPath)
	}
}

func retrieveHourlyValues(client *redis.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Println("Calling redis client to get the data")
		zRevRange := client.ZRevRange("meter-rank", 0, 1)
		var indexRanges []int64
		for i, s := range zRevRange.Val() {
			log.Println("Index, data: ", i, s)
			var readingKeyValue = strings.Split(s, ":")
			msInt, _ := strconv.ParseInt(readingKeyValue[1], 10, 64)
			log.Println("meter-rank position: ", msInt)
			indexRanges = append(indexRanges,msInt)
		}
		startIndex := indexRanges[1]
		endIndex := indexRanges[0]
		log.Println("Start Index, end Index: ", startIndex, endIndex)

		zRevRange1 := client.ZRange("meter-reading", startIndex, endIndex)
		var readings[] Reading

		for i, s := range zRevRange1.Val() {
			log.Println("Values in range", i, s)
			var readingKeyValue = strings.Split(s, ":")
			var value, _ = strconv.ParseFloat(readingKeyValue[1], 32)
			var timeFormatted, _ = msToTime(readingKeyValue[0])
			log.Println("reading value", value)
			readings = append(readings, Reading{timeFormatted, float32(value)})
		}
		log.Println("readings", readings)
		json.NewEncoder(w).Encode(readings)
	}
}

func retrieveMeterValues(client *redis.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Println("Calling redis client to get the data")
		zRevRange := client.ZRevRange("meter-reading", 0, 10)
		var readings[] Reading


		for i, s := range zRevRange.Val() {
			log.Println("Data from redis index, data:", i, s)
			var readingKeyValue = strings.Split(s, ":")
			var value, _ = strconv.ParseFloat(readingKeyValue[1], 32)
			var time, _ = msToTime(readingKeyValue[0])
			log.Println("time, value:", time, value)
			fmt.Printf("Time [%s]", time)
			readings = append(readings, Reading{time, float32(value)})
		}
		log.Println("Readings:", readings)
		fmt.Println(readings)
		json.NewEncoder(w).Encode(readings)
	}

}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func main() {

	var (
		host     = getEnv("REDIS_HOST", "localhost")
		port     = string(getEnv("REDIS_PORT", "6379"))
		password = getEnv("REDIS_PASSWORD", "")
		dataPath     = getEnv("DATA_PATH", "static/")
	)
	log.Println("redis host:", host )
	log.Println("redis port:", port )

	client := redis.NewClient(&redis.Options{
		Addr:     host + ":" + port,
		Password: password,
		DB:       0,
	})


	tm := time.Now()
	hour := tm.Hour()
	var t = time.Date(tm.Year(), tm.Month(), tm.Day(), hour, 10, 0, 0, time.UTC)
	if tm.Minute() > 10 {
		t = t.Add(1 * time.Hour)
	}
	s1 := gocron.NewScheduler(time.UTC)
	j, err := s1.Every(1).Hour().StartAt(t).Do(generateHTML, client, dataPath)
	if err != nil {
		log.Println("error while scheduling", err)
	}
	s1.StartAsync()
	log.Println("next scheduled time:", j.NextRun())



	http.HandleFunc("/", serveFiles(dataPath))
	http.HandleFunc("/index_*", serveFiles(dataPath))
	http.HandleFunc("/data_*", serveFiles(dataPath))
	http.HandleFunc("/recentValues", retrieveMeterValues(client))
	http.HandleFunc("/generateHtml", generateHTMLEndpoint(client, dataPath))
	http.HandleFunc("/fileList", generateFileList(dataPath))
	http.HandleFunc("/dataPastOneHour", retrieveHourlyValues(client))
	http.HandleFunc("/favicon.ico", func(res http.ResponseWriter, req *http.Request) {
		res.Write([]byte{})
	})
	log.Println("Starting server on port: 10002")
	http.ListenAndServe(":10002", nil)
}

func generateHTMLEndpoint(client *redis.Client, dataPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		generateHTML(client, dataPath)
		responseData := "Ok"
		w.Write([]byte(responseData))
	}

}
func serveFiles(dataPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		if p == "/" || p == "" {
			p = dataPath+"/index.html"
		} else {
			p = dataPath+r.URL.Path
		}
		log.Println("url path:", p)

		http.ServeFile(w, r, p)
	}
}