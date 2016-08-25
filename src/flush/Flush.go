package flush

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Section struct {
	StartIdx int64
	EndIdx   int64
	FilePath string // 分块文件完整路径
}

type Downloader struct {
	urlStr        string    // 请求的链接字符串
	sections      []Section // 分块结构体列表
	maxConn       int       // 发起下载的连接数量
	contentLength int64     // 资源大小
	storePath     string    // 文件下载后存储的路径，包括文件名
}

const (
	ACCEPT_RANGES  = "Accept-Ranges"
	CONTENT_LENGTH = "Content-Length"
	RANGE          = "Range"
	LOCATION = "Location"

	KB float64 = 1024
	MB float64 = 1024 * KB
	GB float64 = 1024 * MB
	REDIRECTCODE1 = 301
	REDIRECTCODE2 = 302
	MAXCONN = 1000
)

//type table []string

var (
	logger      *log.Logger
	stopRunChan chan bool
)

func init() {
	logger = log.New(os.Stdout, "", log.Ltime|log.Lmicroseconds/*|log.Lshortfile*/)
	stopRunChan = make(chan bool)
}

// Start 开始执行文件下载
func (dl *Downloader) Start() {
	var (
		finishChan chan bool = make(chan bool)
	)

	go dl.run(finishChan, dl.storePath)

	var (
		desc      string
		size      float64
		tmpLength float64 = float64(dl.contentLength)
	)
	if tmpLength < KB {
		desc = fmt.Sprintf("开始下载[%s] 文件大小%dBytes", filepath.Base(dl.storePath), dl.contentLength)
	} else if tmpLength > KB && tmpLength < MB {
		size = tmpLength / KB
		desc = fmt.Sprintf("开始下载[%s] 文件大小%.2fKB", filepath.Base(dl.storePath), size)
	} else if tmpLength > MB && tmpLength < GB {
		size = tmpLength / MB
		desc = fmt.Sprintf("开始下载[%s] 文件大小%.2fMB", filepath.Base(dl.storePath), size)
	} else {
		size = tmpLength / GB
		desc = fmt.Sprintf("开始下载[%s] 文件大小%.2fGB", filepath.Base(dl.storePath), size)
	}
	logger.Println(desc)

	var wg = new(sync.WaitGroup)
	for _, sec := range dl.sections {
		wg.Add(1)
		go downloadSection(sec, dl.urlStr, wg)
	}

	wg.Wait()
	finishChan <- true
	<-stopRunChan // 阻塞主线程，等待拼接文件完成
}

// downloadSection 下载单个分块文件
func downloadSection(sec Section, urlStr string, wg *sync.WaitGroup) {
	req, err := http.NewRequest("", urlStr, nil)
	if err != nil {
		logger.Fatal("Req:", err)
	}
	filename := filepath.Base(sec.FilePath)

	if sec.EndIdx != int64(0) {
		var value = fmt.Sprintf("bytes=%d-%d", sec.StartIdx, sec.EndIdx)
		req.Header.Add(RANGE, value)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logger.Fatal("Do:", err)
	}

	secFile, err := os.OpenFile(sec.FilePath, os.O_CREATE|os.O_WRONLY, 0777)
	if err != nil {
		logger.Fatalf("创建分块文件%s失败：%s", filename, err)
	}
	defer secFile.Close()

	secBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logger.Fatalf("读取body-%s失败：%s", filename, err)
	}

	_, err = secFile.Write(secBytes)
	if err != nil {
		logger.Fatalf("写入分块文件%s失败：%s", filename, err)
	}

	wg.Done()
}

// run 处理文件是否下载完成
func (dl *Downloader) run(finishChan chan bool, filename string) {
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0777)
	if err != nil {
		logger.Fatalf("创建下载文件%s失败:%s", filename, err)
	}
	defer file.Close()

	tick := time.NewTicker(time.Second)
	defer tick.Stop()

	for {
		select {
		case <-finishChan:
			var sliceSecName = make([]string, dl.maxConn, dl.maxConn)
			for i, sec := range dl.sections {
				sliceSecName[i] = sec.FilePath
			}
			sort.Strings(sliceSecName) // 排序分块文件

			fmt.Println()
			for _, secName := range sliceSecName {
				fmt.Println("正在拼接：", secName)
				err := fileAppend(secName, file) // 这里只是为了能够使用defer close
				if err != nil {
					logger.Fatalf("拼接分块文件%s失败：%s", secName, err)
				}
			}

			removeAllSecFile(sliceSecName)
			fmt.Println()
			logger.Println("下载完成，文件所在路径为：", filename)
			stopRunChan <- true
			return

		case <-tick.C:
			fmt.Print(".")
		}
	}
}

// fileAppend 将分块文件拼接成完整文件
func fileAppend(secName string, file *os.File) interface{} {
	secFile, err := os.Open(secName)
	if err != nil {
		logger.Fatalf("打开分块文件%s失败:%s", secName, err)
	}
	defer secFile.Close()

	_, err = io.Copy(file, secFile)
	return err
}

func New(urlStr string, maxConn int, folder string) (downloader *Downloader) {
	if maxConn > MAXCONN{
		maxConn = MAXCONN	// 限制最大连接数
	}
	downloader = &Downloader{}

	req, err := http.NewRequest("HEAD", urlStr, nil)
	if err != nil {
		logger.Fatal("创建Request失败:", err)
	}

	var resp *http.Response
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		logger.Fatal("创建Response失败：", err)
	}

	// 对重定向进行处理
	if resp.StatusCode == REDIRECTCODE1 || resp.StatusCode == REDIRECTCODE2{
		urlStr = resp.Header.Get(LOCATION)
	}
	fmt.Println("Status:", resp.Status)
	downloader.urlStr = urlStr

	var realUrl *url.URL
	realUrl, err = url.Parse(urlStr)
	if err != nil {
		logger.Fatal("解析Url失败：", err)
	}

	downloader.storePath = genStorePath(folder, realUrl)

	// 判断服务器是否多线程，以及请求的资源大小
	accept := resp.Header.Get(ACCEPT_RANGES)
	if strings.ToLower(accept) == "none" { // 不支持多线程下载
		log.Println("服务器不支持多线程下载，自动切换成单线程下载")
		maxConn = 1
	}
	downloader.maxConn = maxConn

	contentLength := resp.Header.Get(CONTENT_LENGTH)
	downloader.contentLength, err = strconv.ParseInt(contentLength, 10, 64)
	if err != nil {
		log.Println("无法获取需要下载的文件大小，自动切换成单线程下载")
		maxConn = 1
		downloader.contentLength = -1
	}
	tmp := genSections(downloader.contentLength, maxConn, downloader.storePath)
	downloader.sections = make([]Section, maxConn, maxConn)
	copy(downloader.sections, tmp)

	return
}

// genSections 将资源按线程数划分区块
func genSections(cLen int64, maxConn int, fileName string) (section []Section) {
	section = make([]Section, maxConn, maxConn)

	c64 := int64(maxConn)
	cnt := cLen / c64
	for i := int64(0); i < c64; i++ {
		sec := Section{}
		sec.FilePath = fmt.Sprintf("%s.Flush%03d", fileName, i)
		sec.StartIdx = i * cnt
		if i < c64-1 {
			sec.EndIdx = (i+1)*cnt - 1
		} else {
			sec.EndIdx = cLen
		}

		section[i] = sec
	}

	return
}

// genStorePath 生成文件的存储路径
func genStorePath(folder string, realUrl *url.URL) (storePath string) {
	fileName := filepath.Base(realUrl.Path)
	storePath = filepath.Join(folder, fileName)
	if !filepath.IsAbs(storePath) {
		var err interface{}
		storePath, err = filepath.Abs(storePath)
		if err != nil {
			logger.Fatal(err)
		}
	}
	return
}

// removeAllSecFile 将临时的分块文件删除
func removeAllSecFile(sliceSecFiles []string) {
	for _, filename := range sliceSecFiles {
		if filename == "" {
			continue
		}
		os.Remove(filename)
	}
}

//func (t *table) Len() int{
//	return len(t)
//}
//
//func (t *table) Less(i, j int) bool{
//
//}
//
//func (t * table) Swap(i, j int){
//	t[i], t[j] = t[j], t[i]
//}