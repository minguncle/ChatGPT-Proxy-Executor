package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"sync"
	"time"
)

type APIKeys []struct {
	Index  int      `json:"index"`
	Key    string   `json:"key"`
	Type   []string `json:"type"`
	Remark string   `json:"remark"`
}

type Config struct {
	APIKeys         APIKeys `json:"api_keys"`
	ExecutorName    string  `json:"executor_name"`
	SchedulerCenter string  `json:"scheduler_center"`
	ReportEnable    bool    `json:"report_enable"`
	ReportDuration  int     `json:"report_duration"`
	ListenAddr      string  `json:"listen_addr"`
}

type TypeStatus struct {
	Type   string `json:"type"`
	Status string `json:"status"`
}

type APIKeyStatus struct {
	Index      int          `json:"index"`
	Key        string       `json:"key"`
	Usage      float64      `json:"usage"`
	Limit      float64      `json:"limit"`
	Remark     string       `json:"remark"`
	TypeStatus []TypeStatus `json:"type_status"`
}

type SysStatus struct {
	ExecutorName string `json:"executor_name"`
	ExecutorAddr string `json:"executor_addr"`
}

type Status struct {
	APIStatus []APIKeyStatus `json:"api_status"`
	SysStatus SysStatus      `json:"sys_status"`
}

var (
	config *Config
)

const baseUrl = "https://api.openai.com"
const completionUri = "/v1/chat/completions"

func main() {
	var configPath string
	flag.StringVar(&configPath, "c", "", "配置文件路径")
	flag.Parse()
	var err error
	if configPath == "" {
		config, err = loadConfig("config.json")
	} else {
		config, err = loadConfig(configPath)
	}
	if err != nil {
		log.Fatalf("加载配置文件失败: %v", err)
	}
	router := http.NewServeMux()
	router.HandleFunc("/ping", pingHandler)
	router.HandleFunc("/", HandleProxy)
	// 定时上报一次状态
	if config.ReportEnable {
		go func() {
			for {
				reportStatus(config)
				time.Sleep(time.Duration(config.ReportDuration) * time.Second)
			}
		}()
	}
	log.Printf("服务器正在监听端口 %s", config.ListenAddr)
	log.Fatal(http.ListenAndServe(config.ListenAddr, router))
}

func loadConfig(file string) (*Config, error) {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func pingHandler(w http.ResponseWriter, r *http.Request) {
	// 主动获取状态
	//check, err := healthCheck()
	//if err != nil {
	//	return
	//}
	//w.Header().Set("content-type", "application/json")
	_, err := w.Write([]byte("ok"))
	if err != nil {
		return
	}
}

func reportStatus(config *Config) {
	data, err := healthCheck()
	if err != nil {
		log.Println("健康检查失败:", err)
		return
	}

	// 上报到调度中心
	resp, err := http.Post(config.SchedulerCenter, "application/json", bytes.NewBuffer(data))
	if err != nil {
		log.Println("上报状态失败:", err)
		return
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	fmt.Printf("上报状态响应: %s \n", body)
}

func healthCheck() ([]byte, error) {
	// 获取API密钥状态
	apiKeysStatus := make([]APIKeyStatus, len(config.APIKeys))

	for i, apiKey := range config.APIKeys {
		keyStatus, err := getAPIKeyStatus(apiKey.Key, apiKey.Type) // 获取API密钥状态和服务
		if err != nil {
			log.Printf("api status check error: %s", err.Error())
			continue
		}
		apiKeysStatus[i] = APIKeyStatus{
			Index:      i,
			Key:        apiKey.Key,
			TypeStatus: keyStatus,
			Remark:     apiKey.Remark,
		}
	}

	// 创建系统状态切片
	sysStatus := SysStatus{
		ExecutorName: config.ExecutorName,
		ExecutorAddr: config.ListenAddr,
	}

	// 组合API密钥状态和系统状态
	status := Status{
		APIStatus: apiKeysStatus,
		SysStatus: sysStatus,
	}

	// 编码为JSON
	data, err := json.Marshal(status)
	if err != nil {
		log.Println("编码状态失败:", err)
		return nil, err
	}
	return data, err
}

func getAPIKeyStatus(key string, keyTypes []string) ([]TypeStatus, error) {
	// 这里可以添加与OpenAI服务的交互来获取密钥的可用状态和服务
	typeStatusChannel := make(chan TypeStatus, len(keyTypes))
	var wg sync.WaitGroup

	for _, keyType := range keyTypes {
		wg.Add(1)
		go func(key, keyType string) {
			defer wg.Done()
			status, err := checkTypeStatus(key, keyType)
			if err != nil {
				log.Printf("检查密钥类型 %s 的状态时出错: %v", keyType, err)
				return
			}
			typeStatusChannel <- TypeStatus{
				Type:   keyType,
				Status: status,
			}
		}(key, keyType)
	}

	wg.Wait() // 等待所有 goroutine 完成
	close(typeStatusChannel)

	typeStatuses := make([]TypeStatus, 0, len(keyTypes))
	for status := range typeStatusChannel {
		typeStatuses = append(typeStatuses, status)
	}

	return typeStatuses, nil
}

func checkTypeStatus(key string, keyType string) (string, error) {
	// 创建 API 请求
	client := http.Client{
		Timeout: 10 * time.Second,
	}
	req, err := http.NewRequest(http.MethodPost, baseUrl+completionUri, bytes.NewBufferString(fmt.Sprintf("{\n    \"model\": \"%s\"\n}", keyType)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", key))
	req.Header.Set("Content-Type", "application/json")
	rsp, err := client.Do(req)
	if err != nil {
		log.Printf("check gpt status failed! err:%s", err.Error())
		return "inactive", nil
	}
	//msg, _ := io.ReadAll(rsp.Body)
	//if err != nil {
	//	log.Printf("check gpt status failed! err:%s", err.Error())
	//	return "inactive", nil
	//}
	//log.Printf("key [%s] type [%s],check status is %v,msg is %s", key, keyType, rsp.StatusCode, string(msg))
	if rsp.StatusCode != 400 {
		return "inactive", nil
	}
	return "active", nil
}

func HandleProxy(w http.ResponseWriter, r *http.Request) {
	log.Printf("proxy req received! uri:%s", r.URL)
	client := http.DefaultClient
	// 创建 API 请求
	all, _ := io.ReadAll(r.Body)
	log.Printf("req recived with body:[%s]", string(all))
	req, err := http.NewRequest(r.Method, baseUrl+r.URL.Path, bytes.NewBuffer(all))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	req.Header = r.Header
	req.Header.Set("Content-Type", "application/json")
	req.Close = false
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Keep-Alive", "timeout=360")
	rsp, err := client.Do(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rsp.Body.Close()
	//复制 API 响应头部
	for name, values := range rsp.Header {
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}
	w.WriteHeader(rsp.StatusCode)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("access-control-allow-origin", "*")
	// 返回 API 响应主体
	for {
		buff := make([]byte, 32)
		var n int
		n, err = rsp.Body.Read(buff)
		if err != nil {
			break
		}
		_, err = w.Write(buff[:n])
		if err != nil {
			break
		}
		w.(http.Flusher).Flush()
	}

	if err != nil && err != io.EOF {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	return
}
