package main

/**
客户端
*/
import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

/**
客户端
*/
const port = "7777"
const (
	applicationJson string = "application/json"
)

var (
	//存放配置文件的map
	ConfigMap = make(map[string]string)
	//http请求客户端
	Client = &http.Client{}
)

//
func main() {
	//加载配置文件
	//InitConfig()
	//启动客户端连接服务端
	go startClient()
	time.Sleep(time.Hour * 10)
}

/**
socket连接，服务端发给客户端的包装对象
*/
type UrlContext struct {
	Method           string                 `json:"method"`
	RequestUrl       string                 `json:"requestUrl"`
	RequestParam     map[string][]string    `json:"requestParam"`
	RequestHeader    map[string][]string    `json:"requestHeader"`
	JsonRequestParam map[string]interface{} `json:"jsonRequestParam"`
	ApplicationType  string                 `json:"applicationType"`
}

func InitConfig() {
	//下来是配置文件
	InitConfigProperties()
}

/*
加载配置文件
*/
func InitConfigProperties() {
	log.Println("从配置文件目录中加载配置文件开始...")
	f, err := os.Open("conf/config.ini")
	defer f.Close()
	checkErr(err)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		//去掉两边的空格
		lineData := strings.TrimSpace(string(scanner.Text()))
		//去掉注释
		if strings.Contains(lineData, "##") {
			continue
		}
		//按照等号分割
		keyAndValue := strings.Split(lineData, "=")
		if len(keyAndValue) != 2 {
			/*
				如果写两个panic，之后抛一个异常，谁在前面谁就先来.
			*/
			//panic(keyAndValue)
			panic("数据格式不对，一个key必须对应一个value,检测配置文件的个格式，这都能写错，服了你了")
		}
		key := strings.TrimSpace(keyAndValue[0])
		value := strings.TrimSpace(keyAndValue[1])
		if _, ok := ConfigMap[key]; !ok {
			ConfigMap[key] = value
		}
	}
	log.Println("配置文件加载完毕")
}

/**
从命令行加载配置文件命令
*/
func InitCommandLineProperties() {
	log.Println("从命令行加载配置参数")
	//利用flag获取命令行参数
	ipConfig := flag.String("-ipAddr", "", "代理的配置端口")
	flag.Parse()
	fmt.Println("-ipAddr:", *ipConfig)
	if _, ok := ConfigMap["ipAddr"]; !ok {
		ConfigMap["ipAddr"] = *ipConfig
	}
	log.Println("从命令行加载配置参数结束")
}
func startClient() {
	ConfigMap["ipAddr"] = "http://127.0.0.1:8088"
	ConfigMap["serviceAddress"] = "127.0.0.1"
	//启动socket
	address := ConfigMap["serviceAddress"] + ":" + port
	log.Print("socket线程 客户端开始启动，连接服务端地址为:", address)
	tcpAddr, err := net.ResolveTCPAddr("tcp4", address)
	checkErr(err)
	connection, _ := net.DialTCP("tcp", nil, tcpAddr)
	log.Print("socket线程 连接服务端成功")
	//读取 服务端到客户端的连接数据
	scanner := bufio.NewScanner(connection)
	for scanner.Scan() {
		message := scanner.Text()
		log.Println(message)
		//发送请求获取返回值返回
		var s UrlContext
		//使用go自带的json解析
		log.Println("开始解析数据")
		json.Unmarshal([]byte(message), &s)
		//组装参数，发送请求
		param := setRequestParam(&s)
		//响应
		log.Println("响应给客户端")
		fmt.Fprintln(connection, param)
	}
}

//根据socket返回的数据组装成http请求
func setRequestParam(urlContext *UrlContext) string {
	apiUrl := ConfigMap["ipAddr"] + urlContext.RequestUrl
	if urlContext.ApplicationType == applicationJson {
		log.Println("json格式")
		//直接发送post请求 冲
		b, response := sendJsonRequest(apiUrl, urlContext)
		return createResponseReturn(b, response)
	} else {
		switch urlContext.Method {
		case "GET":
			log.Println("get请求")
			b, response := sendRequest("GET", apiUrl, urlContext)
			return createResponseReturn(b, response)
		case "POST":
			log.Println("post请求")
			b, response := sendRequest("POST", apiUrl, urlContext)
			return createResponseReturn(b, response)
		default:
			log.Println("不支持的请求方式:", urlContext)
		}
	}
	return ""
}
func sendJsonRequest(apiUrl string, urlContext *UrlContext) ([]byte, *http.Response) {
	jsonRequestParam, _ := json.Marshal(urlContext.JsonRequestParam)
	log.Println("json的格式为", string(jsonRequestParam))
	body := bytes.NewBuffer(jsonRequestParam)
	log.Println("json请求具体的信息为", urlContext)
	reqest, _ := http.NewRequest("POST", apiUrl, body)
	//设置请求头信息
	setRequestHeader(reqest, urlContext)
	response, _ := Client.Do(reqest)
	defer response.Body.Close()
	b, _ := ioutil.ReadAll(response.Body)
	return b, response
}

/**
发送request请求
*/
func sendRequest(method string, apiUrl string, urlContext *UrlContext) ([]byte, *http.Response) {
	reqest, _ := http.NewRequest(method, apiUrl, nil)
	//设置请求头信息
	setRequestHeader(reqest, urlContext)
	data := make(url.Values)
	for k, v := range urlContext.RequestParam {
		//设置参数
		for _, s := range v {
			data.Set(k, s)
		}
	}
	reqest.URL.RawQuery = data.Encode()
	url.ParseRequestURI(apiUrl)
	log.Println("开始调用")
	response, _ := Client.Do(reqest)
	log.Println("调用结束")
	defer response.Body.Close()
	log.Println("读取信息")
	b, _ := ioutil.ReadAll(response.Body)
	log.Println("返回信息")
	return b, response
}

//构建返回值返回给客户端
func createResponseReturn(c []byte, r *http.Response) string {
	log.Println("构建返回信息")
	responseBody := new(ResponseBody)
	responseBody.Init(r.Header, string(c))
	return responseBody.TransformJson()
}

/**
客户端发送给服务端的消息的数据格式
*/
type ResponseBody struct {
	MyHeader map[string][]string `json:"myHeader"`
	MyBody   string              `json:"myBody"`
}

//初始化客户端
func (this *ResponseBody) Init(header map[string][]string, body string) {
	this.MyHeader = header
	this.MyBody = body
}

//构建json返回
func (this *ResponseBody) TransformJson() string {
	bf := bytes.NewBuffer([]byte{})
	jsonEncoder := json.NewEncoder(bf)
	jsonEncoder.SetEscapeHTML(false)
	jsonEncoder.Encode(this)
	return bf.String()
}

/**
设置请求头
*/
func setRequestHeader(request *http.Request, urlContext *UrlContext) {
	for k, v := range urlContext.RequestHeader {
		//设置参数
		for _, s := range v {
			request.Header.Set(k, s)
		}
	}
}

func startHttpClient() {
	http.HandleFunc("/test", sayhelloName)   //设置访问的路由
	err := http.ListenAndServe(":9091", nil) //设置监听的端口
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
func sayhelloName(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	log.Println(r.FormValue("userName"))
	log.Println(r.FormValue("passWord"))
	r2 := R{
		Code: 200,
		Msg:  "success",
		Data: r.FormValue("userName"),
	}
	b, _ := json.Marshal(r2)
	w.Header().Set("Content-Type", "text/json; charset=utf-8")
	w.Write(b)
}
func checkErr(err error) {
	if err != nil {
		log.Fatalln(err)
	}
}

type R struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
}
