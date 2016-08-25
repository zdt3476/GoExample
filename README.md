## GoExample
用Go实现的一些Demo

### 例子列表

#### GitCheat
> Desc: 
>> 制造假的Contributions,仅用于娱乐    

> Usage: 
>> 下载Src/GitCheat.go, 编译(go build GitCheat.go), 将生成的可执行文件放到你的项目目录下
>> 执行(GitCheat 2016-01-24 2016-03-10)，最后push你的项目即可

#### Flush
> Desc:
>> 一个简单的多线程下载器

> Usage:
```Go
runtime.GOMAXPROCS(runtime.NumCPU()-1)
dl := flush.New("https://zdt3476.github.io/background/resume-no20.pdf", 20, "D:\\")
dl.Start()
```
