package main

import (
	"container/list"
	"fmt"
	"os"
	"time"
	"math/rand"
	"os/exec"
)

const (
	USAGE string = "用法：GitCheat 2016-08-24 2016-09-10"
	ONEDAY = 60*60*24*time.Second
	MAXDELTADAY = 4
	MAXCOMMITNUM = 20
	GITFILENAME = "cheat"
	GITADDSTRING = "git add ./" + GITFILENAME
	GITCOMMITFMT = "git commit --date=%s -m \"modify content to %d\""
)

var myRand = rand.New(rand.NewSource(time.Now().UnixNano()))

func main() {
	if len(os.Args) < 3 {
		fmt.Println(USAGE)
		os.Exit(-1)
	}

	startDate, endDate := parseDate()
	lst := genDateList(startDate, endDate)

	doCheat(lst)
}

// doCheat 开始制造假的commit
func doCheat(lst *list.List){
	for day := lst.Front(); day != nil; day = day.Next(){
		var num = myRand.Intn(MAXCOMMITNUM) // 每日的随机提交次数

		for cnt := 0; cnt <= num; cnt++{
			file, err := os.OpenFile(GITFILENAME, os.O_CREATE | os.O_WRONLY, 0666)
			if err != nil{
				fmt.Println("创建文件失败")
				os.Exit(-1)
			}
			var content = myRand.Int()
			file.WriteString(fmt.Sprintf("%d", content))
			file.Close()

			execGitCmd(GITADDSTRING)
			commit := fmt.Sprintf(GITCOMMITFMT, day, content)
			execGitCmd(commit)

			fmt.Println(commit)
		}
	}
}

// execGitCmd 执行Git命令
func execGitCmd(cmdName string){
	cmd := exec.Command(cmdName, "")
	err := cmd.Start()
	if err != nil{
		fmt.Println(err)
		os.Exit(-1)
	}
}

// parseDate 解析开始时间和结束时间
func parseDate() (startDate, endDate time.Time) {
	var err interface{}

	startDate, err = time.Parse("2006-01-02", os.Args[1])
	if err != nil {
		fmt.Println(err, USAGE)
		os.Exit(-1)
	}
	endDate, err = time.Parse("2006-01-02", os.Args[2])
	if err != nil {
		fmt.Println(err, USAGE)
		os.Exit(-1)
	}

	return
}

// genDateList 生成一个日期列表
func genDateList(start, end time.Time) (lstDate *list.List) {
	lstDate = list.New()
	lstDate.PushBack(start)

	var (
		tmp = start
		delta int
		deltaTime time.Duration
	)

	for tmp.Before(end) {
		delta = myRand.Intn(MAXDELTADAY)
		if delta > 0{
			deltaTime = ONEDAY * time.Duration(delta)
		}else {
			deltaTime = ONEDAY
		}
		tmp  = tmp.Add(deltaTime)
		lstDate.PushBack(tmp)
	}

	if !tmp.Equal(end){
		lstDate.PushBack(end)
	}

	return
}